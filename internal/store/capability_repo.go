package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/open-fin/agent-composer/internal/domain"
)

type pgCapabilities struct{ p *Postgres }

const capabilityColumns = `id, slug, name, type, subtype, description, source_system,
	external_id, business_domain, intents_json, tags_json, input_schema_json,
	output_schema_json, owner, permissions_json, risk_level, reusable, status,
	metadata_json, created_at, updated_at`

func (r *pgCapabilities) Create(ctx context.Context, c domain.Capability) (domain.Capability, error) {
	if c.ID == "" {
		c.ID = domain.NewID(c.Slug)
	}
	cols, err := capabilityJSONColumns(c)
	if err != nil {
		return domain.Capability{}, err
	}
	row := r.p.pool.QueryRow(ctx, `
		INSERT INTO capabilities
			(id, slug, name, type, subtype, description, source_system, external_id,
			 business_domain, intents_json, tags_json, input_schema_json, output_schema_json,
			 owner, permissions_json, risk_level, reusable, status, metadata_json)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
		RETURNING `+capabilityColumns,
		c.ID, c.Slug, c.Name, c.Type, c.Subtype, c.Description, c.SourceSystem, c.ExternalID,
		c.BusinessDomain, cols.intents, cols.tags, cols.input, cols.output,
		c.Owner, cols.permissions, c.RiskLevel, c.Reusable, c.Status, cols.metadata)
	out, err := scanCapability(row)
	return out, translate(err, fmt.Sprintf("capability %s/%s", c.Type, c.Slug))
}

func (r *pgCapabilities) Get(ctx context.Context, id string) (domain.Capability, error) {
	row := r.p.pool.QueryRow(ctx, `SELECT `+capabilityColumns+` FROM capabilities WHERE id = $1`, id)
	out, err := scanCapability(row)
	return out, translate(err, fmt.Sprintf("capability %s", id))
}

func (r *pgCapabilities) GetBySlug(ctx context.Context, capType domain.CapabilityType, slug string) (domain.Capability, error) {
	row := r.p.pool.QueryRow(ctx, `SELECT `+capabilityColumns+`
		FROM capabilities WHERE type = $1 AND slug = $2`, capType, slug)
	out, err := scanCapability(row)
	return out, translate(err, fmt.Sprintf("capability %s/%s", capType, slug))
}

// FindByExternalID resolves an extraction-time dependency reference. A capability
// imported from the same source system wins; otherwise any source that carries the
// same external id is accepted, so a tool declared in two Dify apps resolves once.
func (r *pgCapabilities) FindByExternalID(ctx context.Context, sourceSystem, externalID string) (domain.Capability, error) {
	row := r.p.pool.QueryRow(ctx, `SELECT `+capabilityColumns+`
		FROM capabilities WHERE external_id = $1 AND external_id <> ''
		ORDER BY (source_system = $2) DESC, created_at ASC
		LIMIT 1`, externalID, sourceSystem)
	out, err := scanCapability(row)
	return out, translate(err, fmt.Sprintf("capability external_id=%s", externalID))
}

func (r *pgCapabilities) List(ctx context.Context, f CapabilityFilter) ([]domain.Capability, Page, error) {
	var (
		args  []any
		where []string
	)
	add := func(format string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(format, len(args)))
	}
	if f.Type != "" {
		add("type = $%d", f.Type)
	}
	if f.Status != "" {
		add("status = $%d", f.Status)
	}
	if f.Domain != "" {
		add("business_domain = $%d", f.Domain)
	}
	if f.Query != "" {
		args = append(args, f.Query)
		n := len(args)
		// Search covers display text plus the matchable vocabulary (intents, tags).
		where = append(where, fmt.Sprintf(
			`(name ILIKE '%%' || $%d || '%%' OR description ILIKE '%%' || $%d || '%%'
			  OR slug ILIKE '%%' || $%d || '%%'
			  OR intents_json::text ILIKE '%%' || $%d || '%%'
			  OR tags_json::text ILIKE '%%' || $%d || '%%')`, n, n, n, n, n))
	}
	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.p.pool.QueryRow(ctx, `SELECT count(*) FROM capabilities`+clause, args...).Scan(&total); err != nil {
		return nil, Page{}, translate(err, "capabilities")
	}
	page := Page{Total: total, Limit: f.Limit, Offset: f.Offset}
	page.Normalize()
	args = append(args, page.Limit, page.Offset)
	query := `SELECT ` + capabilityColumns + ` FROM capabilities` + clause +
		fmt.Sprintf(" ORDER BY created_at ASC, id ASC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	rows, err := r.p.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, Page{}, translate(err, "capabilities")
	}
	defer rows.Close()
	out := []domain.Capability{}
	for rows.Next() {
		c, err := scanCapability(rows)
		if err != nil {
			return nil, Page{}, err
		}
		out = append(out, c)
	}
	return out, page, rows.Err()
}

func (r *pgCapabilities) Update(ctx context.Context, c domain.Capability) (domain.Capability, error) {
	cols, err := capabilityJSONColumns(c)
	if err != nil {
		return domain.Capability{}, err
	}
	row := r.p.pool.QueryRow(ctx, `
		UPDATE capabilities SET
			slug = $2, name = $3, type = $4, subtype = $5, description = $6,
			source_system = $7, external_id = $8, business_domain = $9,
			intents_json = $10, tags_json = $11, input_schema_json = $12,
			output_schema_json = $13, owner = $14, permissions_json = $15,
			risk_level = $16, reusable = $17, status = $18, metadata_json = $19,
			updated_at = now()
		WHERE id = $1
		RETURNING `+capabilityColumns,
		c.ID, c.Slug, c.Name, c.Type, c.Subtype, c.Description, c.SourceSystem, c.ExternalID,
		c.BusinessDomain, cols.intents, cols.tags, cols.input, cols.output,
		c.Owner, cols.permissions, c.RiskLevel, c.Reusable, c.Status, cols.metadata)
	out, err := scanCapability(row)
	return out, translate(err, fmt.Sprintf("capability %s/%s", c.Type, c.Slug))
}

type capabilityJSON struct {
	intents, tags, input, output, permissions, metadata []byte
}

func capabilityJSONColumns(c domain.Capability) (capabilityJSON, error) {
	var (
		cols capabilityJSON
		err  error
	)
	if cols.intents, err = jsonArray(c.Intents); err != nil {
		return cols, err
	}
	if cols.tags, err = jsonArray(c.Tags); err != nil {
		return cols, err
	}
	if cols.input, err = jsonObject(c.InputSchema); err != nil {
		return cols, err
	}
	if cols.output, err = jsonObject(c.OutputSchema); err != nil {
		return cols, err
	}
	if cols.permissions, err = jsonArray(c.Permissions); err != nil {
		return cols, err
	}
	if cols.metadata, err = jsonObject(c.Metadata); err != nil {
		return cols, err
	}
	return cols, nil
}

func scanCapability(row rowScanner) (domain.Capability, error) {
	var (
		c    domain.Capability
		cols capabilityJSON
	)
	if err := row.Scan(&c.ID, &c.Slug, &c.Name, &c.Type, &c.Subtype, &c.Description,
		&c.SourceSystem, &c.ExternalID, &c.BusinessDomain, &cols.intents, &cols.tags,
		&cols.input, &cols.output, &c.Owner, &cols.permissions, &c.RiskLevel,
		&c.Reusable, &c.Status, &cols.metadata, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return domain.Capability{}, err
	}
	c.Intents, c.Tags, c.Permissions = []string{}, []string{}, []string{}
	c.InputSchema, c.OutputSchema, c.Metadata = map[string]any{}, map[string]any{}, map[string]any{}
	for _, pair := range []struct {
		raw []byte
		dst any
	}{
		{cols.intents, &c.Intents}, {cols.tags, &c.Tags}, {cols.permissions, &c.Permissions},
		{cols.input, &c.InputSchema}, {cols.output, &c.OutputSchema}, {cols.metadata, &c.Metadata},
	} {
		if err := decodeJSON(pair.raw, pair.dst); err != nil {
			return domain.Capability{}, err
		}
	}
	return c, nil
}
