package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/open-fin/agent-composer/internal/domain"
)

type pgDrafts struct{ p *Postgres }

const draftColumns = `id, slug, name, goal, description, harness_type, status,
	composition_json, governance_json, yaml_text, created_at, updated_at`

func (r *pgDrafts) Create(ctx context.Context, d domain.BusinessAgentDraft, components []domain.CompositionComponent) (domain.BusinessAgentDraft, error) {
	if d.ID == "" {
		d.ID = domain.NewID(d.Slug)
	}
	composition, governance, err := draftJSON(d)
	if err != nil {
		return domain.BusinessAgentDraft{}, err
	}

	tx, err := r.p.pool.Begin(ctx)
	if err != nil {
		return domain.BusinessAgentDraft{}, translate(err, "draft")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		INSERT INTO business_agent_drafts
			(id, slug, name, goal, description, harness_type, status,
			 composition_json, governance_json, yaml_text)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING `+draftColumns,
		d.ID, d.Slug, d.Name, d.Goal, d.Description, d.HarnessType, d.Status,
		composition, governance, d.YAMLText)
	out, err := scanDraft(row)
	if err != nil {
		return domain.BusinessAgentDraft{}, translate(err, fmt.Sprintf("draft %s", d.Slug))
	}
	if err := replaceComponents(ctx, tx, out.ID, components); err != nil {
		return domain.BusinessAgentDraft{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.BusinessAgentDraft{}, translate(err, "draft")
	}
	out.Components, err = r.componentsFor(ctx, out.ID)
	return out, err
}

func (r *pgDrafts) Get(ctx context.Context, id string) (domain.BusinessAgentDraft, error) {
	row := r.p.pool.QueryRow(ctx, `SELECT `+draftColumns+` FROM business_agent_drafts WHERE id = $1`, id)
	out, err := scanDraft(row)
	if err != nil {
		return domain.BusinessAgentDraft{}, translate(err, fmt.Sprintf("draft %s", id))
	}
	out.Components, err = r.componentsFor(ctx, id)
	return out, err
}

func (r *pgDrafts) List(ctx context.Context, f DraftFilter) ([]domain.BusinessAgentDraft, Page, error) {
	var (
		args  []any
		where []string
	)
	if f.Status != "" {
		args = append(args, f.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if f.Query != "" {
		args = append(args, f.Query)
		n := len(args)
		where = append(where, fmt.Sprintf(
			`(name ILIKE '%%' || $%d || '%%' OR goal ILIKE '%%' || $%d || '%%' OR slug ILIKE '%%' || $%d || '%%')`, n, n, n))
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.p.pool.QueryRow(ctx, `SELECT count(*) FROM business_agent_drafts`+clause, args...).Scan(&total); err != nil {
		return nil, Page{}, translate(err, "drafts")
	}
	page := Page{Total: total, Limit: f.Limit, Offset: f.Offset}
	page.Normalize()
	args = append(args, page.Limit, page.Offset)

	rows, err := r.p.pool.Query(ctx, `SELECT `+draftColumns+` FROM business_agent_drafts`+clause+
		fmt.Sprintf(" ORDER BY created_at ASC, id ASC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return nil, Page{}, translate(err, "drafts")
	}
	defer rows.Close()
	out := []domain.BusinessAgentDraft{}
	for rows.Next() {
		d, err := scanDraft(rows)
		if err != nil {
			return nil, Page{}, err
		}
		out = append(out, d)
	}
	return out, page, rows.Err()
}

func (r *pgDrafts) Update(ctx context.Context, d domain.BusinessAgentDraft, components []domain.CompositionComponent) (domain.BusinessAgentDraft, error) {
	composition, governance, err := draftJSON(d)
	if err != nil {
		return domain.BusinessAgentDraft{}, err
	}
	tx, err := r.p.pool.Begin(ctx)
	if err != nil {
		return domain.BusinessAgentDraft{}, translate(err, "draft")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		UPDATE business_agent_drafts SET
			slug = $2, name = $3, goal = $4, description = $5, harness_type = $6,
			status = $7, composition_json = $8, governance_json = $9, yaml_text = $10,
			updated_at = now()
		WHERE id = $1
		RETURNING `+draftColumns,
		d.ID, d.Slug, d.Name, d.Goal, d.Description, d.HarnessType, d.Status,
		composition, governance, d.YAMLText)
	out, err := scanDraft(row)
	if err != nil {
		return domain.BusinessAgentDraft{}, translate(err, fmt.Sprintf("draft %s", d.ID))
	}
	// A nil slice means "leave the composition alone"; an empty slice clears it.
	if components != nil {
		if err := replaceComponents(ctx, tx, out.ID, components); err != nil {
			return domain.BusinessAgentDraft{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.BusinessAgentDraft{}, translate(err, "draft")
	}
	out.Components, err = r.componentsFor(ctx, out.ID)
	return out, err
}

// replaceComponents rewrites a draft's authoritative composition rows inside the
// caller's transaction. order_index is assigned from slice position, so YAML rendering
// and the UI always agree on ordering.
func replaceComponents(ctx context.Context, tx pgx.Tx, draftID string, components []domain.CompositionComponent) error {
	if _, err := tx.Exec(ctx, `DELETE FROM composition_components WHERE draft_id = $1`, draftID); err != nil {
		return translate(err, "composition components")
	}
	for i, comp := range components {
		if comp.ID == "" {
			comp.ID = domain.NewID("component")
		}
		config, err := jsonObject(comp.Config)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO composition_components
				(id, draft_id, capability_id, component_role, order_index, config_json)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (draft_id, capability_id, component_role) DO NOTHING`,
			comp.ID, draftID, comp.CapabilityID, comp.Role, i, config); err != nil {
			return translate(err, "composition component")
		}
	}
	return nil
}

func (r *pgDrafts) componentsFor(ctx context.Context, draftID string) ([]domain.CompositionComponent, error) {
	rows, err := r.p.pool.Query(ctx, `
		SELECT cc.id, cc.draft_id, cc.capability_id, cc.component_role, cc.order_index,
		       cc.config_json, cc.created_at, c.slug, c.name, c.type
		FROM composition_components cc
		JOIN capabilities c ON c.id = cc.capability_id
		WHERE cc.draft_id = $1
		ORDER BY cc.order_index ASC, cc.id ASC`, draftID)
	if err != nil {
		return nil, translate(err, "composition components")
	}
	defer rows.Close()
	out := []domain.CompositionComponent{}
	for rows.Next() {
		var (
			comp   domain.CompositionComponent
			config []byte
		)
		if err := rows.Scan(&comp.ID, &comp.DraftID, &comp.CapabilityID, &comp.Role,
			&comp.OrderIndex, &config, &comp.CreatedAt,
			&comp.CapabilitySlug, &comp.CapabilityName, &comp.CapabilityType); err != nil {
			return nil, err
		}
		comp.Config = map[string]any{}
		if err := decodeJSON(config, &comp.Config); err != nil {
			return nil, err
		}
		out = append(out, comp)
	}
	return out, rows.Err()
}

func scanDraft(row rowScanner) (domain.BusinessAgentDraft, error) {
	var (
		d                       domain.BusinessAgentDraft
		composition, governance []byte
	)
	if err := row.Scan(&d.ID, &d.Slug, &d.Name, &d.Goal, &d.Description, &d.HarnessType,
		&d.Status, &composition, &governance, &d.YAMLText, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return domain.BusinessAgentDraft{}, err
	}
	if err := decodeJSON(composition, &d.Composition); err != nil {
		return domain.BusinessAgentDraft{}, err
	}
	if err := decodeJSON(governance, &d.Governance); err != nil {
		return domain.BusinessAgentDraft{}, err
	}
	return d, nil
}

func draftJSON(d domain.BusinessAgentDraft) (composition, governance []byte, err error) {
	if composition, err = jsonObject(d.Composition); err != nil {
		return nil, nil, err
	}
	governance, err = jsonObject(d.Governance)
	return composition, governance, err
}
