package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/open-fin/agent-composer/internal/domain"
)

type pgCandidates struct{ p *Postgres }

const candidateColumns = `id, source_id, source_system, external_id, candidate_type, name,
	description, raw_payload_json, extracted_json, inferred_json, review_json, status,
	created_at, updated_at`

// Upsert keys on (source_id, external_id). Re-importing a DSL refreshes the extracted
// and inferred fields while preserving the candidate's identity, any reviewer edits,
// and any terminal decision already taken.
func (r *pgCandidates) Upsert(ctx context.Context, c domain.CapabilityCandidate) (domain.CapabilityCandidate, error) {
	if c.ID == "" {
		c.ID = domain.NewID(c.Name)
	}
	raw, err := jsonObject(c.RawPayload)
	if err != nil {
		return domain.CapabilityCandidate{}, err
	}
	extracted, err := jsonObject(c.Extracted)
	if err != nil {
		return domain.CapabilityCandidate{}, err
	}
	inferred, err := jsonObject(c.Inferred)
	if err != nil {
		return domain.CapabilityCandidate{}, err
	}
	review, err := jsonObject(c.Review)
	if err != nil {
		return domain.CapabilityCandidate{}, err
	}

	row := r.p.pool.QueryRow(ctx, `
		INSERT INTO capability_candidates
			(id, source_id, source_system, external_id, candidate_type, name, description,
			 raw_payload_json, extracted_json, inferred_json, review_json, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (source_id, external_id) DO UPDATE SET
			candidate_type   = EXCLUDED.candidate_type,
			name             = EXCLUDED.name,
			description      = EXCLUDED.description,
			raw_payload_json = EXCLUDED.raw_payload_json,
			extracted_json   = EXCLUDED.extracted_json,
			inferred_json    = EXCLUDED.inferred_json,
			status           = EXCLUDED.status,
			updated_at       = now()
		WHERE capability_candidates.status IN ('extracted', 'inferred', 'needs_review')
		RETURNING `+candidateColumns,
		c.ID, c.SourceID, c.SourceSystem, c.ExternalID, c.CandidateType, c.Name, c.Description,
		raw, extracted, inferred, review, c.Status)

	out, err := scanCandidate(row)
	if err != nil {
		translated := translate(err, fmt.Sprintf("candidate %s", c.ExternalID))
		if isNotFound(translated) {
			// The DO UPDATE ... WHERE guard filtered the row out: the candidate is
			// already registered or rejected, so leave it untouched and return it.
			return r.getByExternal(ctx, c.SourceID, c.ExternalID)
		}
		return domain.CapabilityCandidate{}, translated
	}
	return out, nil
}

func (r *pgCandidates) getByExternal(ctx context.Context, sourceID, externalID string) (domain.CapabilityCandidate, error) {
	row := r.p.pool.QueryRow(ctx, `SELECT `+candidateColumns+`
		FROM capability_candidates WHERE source_id = $1 AND external_id = $2`, sourceID, externalID)
	out, err := scanCandidate(row)
	return out, translate(err, fmt.Sprintf("candidate %s", externalID))
}

func (r *pgCandidates) Get(ctx context.Context, id string) (domain.CapabilityCandidate, error) {
	row := r.p.pool.QueryRow(ctx, `SELECT `+candidateColumns+` FROM capability_candidates WHERE id = $1`, id)
	out, err := scanCandidate(row)
	return out, translate(err, fmt.Sprintf("candidate %s", id))
}

func (r *pgCandidates) List(ctx context.Context, f CandidateFilter) ([]domain.CapabilityCandidate, Page, error) {
	var (
		args  []any
		where []string
	)
	add := func(format string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(format, len(args)))
	}
	if f.Status != "" {
		add("status = $%d", f.Status)
	}
	if f.Type != "" {
		add("candidate_type = $%d", f.Type)
	}
	if f.SourceID != "" {
		add("source_id = $%d", f.SourceID)
	}
	if f.Query != "" {
		args = append(args, f.Query)
		n := len(args)
		where = append(where, fmt.Sprintf(
			`(name ILIKE '%%' || $%d || '%%' OR description ILIKE '%%' || $%d || '%%'
			  OR external_id ILIKE '%%' || $%d || '%%')`, n, n, n))
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.p.pool.QueryRow(ctx, `SELECT count(*) FROM capability_candidates`+clause, args...).Scan(&total); err != nil {
		return nil, Page{}, translate(err, "candidates")
	}

	page := Page{Total: total, Limit: f.Limit, Offset: f.Offset}
	page.Normalize()
	args = append(args, page.Limit, page.Offset)
	query := `SELECT ` + candidateColumns + ` FROM capability_candidates` + clause +
		fmt.Sprintf(" ORDER BY created_at ASC, id ASC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, Page{}, translate(err, "candidates")
	}
	defer rows.Close()
	out := []domain.CapabilityCandidate{}
	for rows.Next() {
		cand, err := scanCandidate(rows)
		if err != nil {
			return nil, Page{}, err
		}
		out = append(out, cand)
	}
	return out, page, rows.Err()
}

func (r *pgCandidates) Update(ctx context.Context, c domain.CapabilityCandidate) (domain.CapabilityCandidate, error) {
	extracted, err := jsonObject(c.Extracted)
	if err != nil {
		return domain.CapabilityCandidate{}, err
	}
	inferred, err := jsonObject(c.Inferred)
	if err != nil {
		return domain.CapabilityCandidate{}, err
	}
	review, err := jsonObject(c.Review)
	if err != nil {
		return domain.CapabilityCandidate{}, err
	}
	row := r.p.pool.QueryRow(ctx, `
		UPDATE capability_candidates SET
			name = $2, description = $3, candidate_type = $4,
			extracted_json = $5, inferred_json = $6, review_json = $7,
			status = $8, updated_at = now()
		WHERE id = $1
		RETURNING `+candidateColumns,
		c.ID, c.Name, c.Description, c.CandidateType, extracted, inferred, review, c.Status)
	out, err := scanCandidate(row)
	return out, translate(err, fmt.Sprintf("candidate %s", c.ID))
}

func scanCandidate(row rowScanner) (domain.CapabilityCandidate, error) {
	var (
		c                                      domain.CapabilityCandidate
		raw, extracted, inferred, reviewFields []byte
	)
	if err := row.Scan(&c.ID, &c.SourceID, &c.SourceSystem, &c.ExternalID, &c.CandidateType,
		&c.Name, &c.Description, &raw, &extracted, &inferred, &reviewFields, &c.Status,
		&c.CreatedAt, &c.UpdatedAt); err != nil {
		return domain.CapabilityCandidate{}, err
	}
	c.RawPayload = map[string]any{}
	if err := decodeJSON(raw, &c.RawPayload); err != nil {
		return domain.CapabilityCandidate{}, err
	}
	if err := decodeJSON(extracted, &c.Extracted); err != nil {
		return domain.CapabilityCandidate{}, err
	}
	if err := decodeJSON(inferred, &c.Inferred); err != nil {
		return domain.CapabilityCandidate{}, err
	}
	if err := decodeJSON(reviewFields, &c.Review); err != nil {
		return domain.CapabilityCandidate{}, err
	}
	return c, nil
}

func isNotFound(err error) bool { return errors.Is(err, ErrNotFound) }
