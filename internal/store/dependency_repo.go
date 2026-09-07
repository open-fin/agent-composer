package store

import (
	"context"
	"fmt"

	"github.com/open-fin/agent-composer/internal/domain"
)

type pgDependencies struct{ p *Postgres }

// Replace rewrites a capability's outgoing edges in one transaction, so registration is
// idempotent: re-registering never leaves stale dependencies behind.
func (r *pgDependencies) Replace(ctx context.Context, capabilityID string, deps []domain.Dependency) error {
	tx, err := r.p.pool.Begin(ctx)
	if err != nil {
		return translate(err, "dependencies")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM capability_dependencies WHERE capability_id = $1`, capabilityID); err != nil {
		return translate(err, "dependencies")
	}
	for _, dep := range deps {
		if dep.DependsOnCapabilityID == "" || dep.DependsOnCapabilityID == capabilityID {
			continue
		}
		if dep.ID == "" {
			dep.ID = domain.NewID("dep")
		}
		config, err := jsonObject(dep.Config)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO capability_dependencies
				(id, capability_id, depends_on_capability_id, dependency_type, config_json)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (capability_id, depends_on_capability_id, dependency_type) DO NOTHING`,
			dep.ID, capabilityID, dep.DependsOnCapabilityID, dep.DependencyType, config); err != nil {
			return translate(err, "dependency")
		}
	}
	return translate(tx.Commit(ctx), "dependencies")
}

func (r *pgDependencies) ListFor(ctx context.Context, capabilityID string) ([]domain.Dependency, error) {
	rows, err := r.p.pool.Query(ctx, `
		SELECT d.id, d.capability_id, d.depends_on_capability_id, d.dependency_type,
		       d.config_json, d.created_at, c.slug, c.type, c.name
		FROM capability_dependencies d
		JOIN capabilities c ON c.id = d.depends_on_capability_id
		WHERE d.capability_id = $1
		ORDER BY d.created_at ASC, d.id ASC`, capabilityID)
	if err != nil {
		return nil, translate(err, fmt.Sprintf("dependencies of %s", capabilityID))
	}
	defer rows.Close()

	out := []domain.Dependency{}
	for rows.Next() {
		var (
			dep    domain.Dependency
			config []byte
		)
		if err := rows.Scan(&dep.ID, &dep.CapabilityID, &dep.DependsOnCapabilityID,
			&dep.DependencyType, &config, &dep.CreatedAt,
			&dep.DependsOnSlug, &dep.DependsOnType, &dep.DependsOnName); err != nil {
			return nil, err
		}
		dep.Config = map[string]any{}
		if err := decodeJSON(config, &dep.Config); err != nil {
			return nil, err
		}
		out = append(out, dep)
	}
	return out, rows.Err()
}
