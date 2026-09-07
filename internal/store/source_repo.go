package store

import (
	"context"
	"fmt"

	"github.com/open-fin/agent-composer/internal/domain"
)

type pgSources struct{ p *Postgres }

const sourceColumns = `id, name, type, base_url, auth_type, config_json, status, created_at, updated_at`

func (r *pgSources) Create(ctx context.Context, src domain.ExternalSource) (domain.ExternalSource, error) {
	if src.ID == "" {
		src.ID = domain.NewID(src.Name)
	}
	if src.Status == "" {
		src.Status = "active"
	}
	config, err := jsonObject(src.Config)
	if err != nil {
		return domain.ExternalSource{}, err
	}
	row := r.p.pool.QueryRow(ctx, `
		INSERT INTO external_sources (id, name, type, base_url, auth_type, config_json, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+sourceColumns,
		src.ID, src.Name, src.Type, src.BaseURL, src.AuthType, config, src.Status)
	out, err := scanSource(row)
	return out, translate(err, fmt.Sprintf("source %s", src.Name))
}

func (r *pgSources) Get(ctx context.Context, id string) (domain.ExternalSource, error) {
	row := r.p.pool.QueryRow(ctx, `SELECT `+sourceColumns+` FROM external_sources WHERE id = $1`, id)
	out, err := scanSource(row)
	return out, translate(err, fmt.Sprintf("source %s", id))
}

func (r *pgSources) List(ctx context.Context) ([]domain.ExternalSource, error) {
	rows, err := r.p.pool.Query(ctx, `SELECT `+sourceColumns+` FROM external_sources ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, translate(err, "sources")
	}
	defer rows.Close()
	out := []domain.ExternalSource{}
	for rows.Next() {
		src, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

// EnsureByName is the path every import takes: an upload names its source and gets it
// created on first use, so no configuration step is required before importing.
func (r *pgSources) EnsureByName(ctx context.Context, src domain.ExternalSource) (domain.ExternalSource, error) {
	row := r.p.pool.QueryRow(ctx, `SELECT `+sourceColumns+`
		FROM external_sources WHERE name = $1 AND type = $2`, src.Name, src.Type)
	existing, err := scanSource(row)
	if err == nil {
		return existing, nil
	}
	if translated := translate(err, "source"); !isNotFound(translated) {
		return domain.ExternalSource{}, translated
	}
	return r.Create(ctx, src)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSource(row rowScanner) (domain.ExternalSource, error) {
	var (
		src    domain.ExternalSource
		config []byte
	)
	if err := row.Scan(&src.ID, &src.Name, &src.Type, &src.BaseURL, &src.AuthType,
		&config, &src.Status, &src.CreatedAt, &src.UpdatedAt); err != nil {
		return domain.ExternalSource{}, err
	}
	src.Config = map[string]any{}
	if err := decodeJSON(config, &src.Config); err != nil {
		return domain.ExternalSource{}, err
	}
	return src, nil
}
