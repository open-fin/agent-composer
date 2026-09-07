package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-fin/agent-composer/migrations"
)

// Postgres is the durable Store implementation.
type Postgres struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// NewPostgres connects to dsn, retrying while the database container comes up, and
// optionally applies the embedded migrations.
func NewPostgres(ctx context.Context, dsn string, maxConns int32, attempts int, migrateOnStart bool, log *slog.Logger) (*Postgres, error) {
	if log == nil {
		log = slog.Default()
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	if attempts <= 0 {
		attempts = 1
	}
	var pingErr error
	for i := 0; i < attempts; i++ {
		if pingErr = pool.Ping(ctx); pingErr == nil {
			break
		}
		if ctx.Err() != nil {
			break
		}
		log.Info("waiting for postgres", "attempt", i+1, "of", attempts)
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
	}
	if pingErr != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to postgres: %w", pingErr)
	}

	pg := &Postgres{pool: pool, log: log}
	if migrateOnStart {
		if err := pg.Migrate(ctx); err != nil {
			pool.Close()
			return nil, fmt.Errorf("apply migrations: %w", err)
		}
	}
	return pg, nil
}

// Migrate applies every embedded migration that has not run yet, in filename order.
func (p *Postgres) Migrate(ctx context.Context) error {
	if _, err := p.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var applied bool
		if err := p.pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name).
			Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied {
			continue
		}
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		tx, err := p.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("exec migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
		p.log.Info("applied migration", "version", name)
	}
	return nil
}

func (p *Postgres) Sources() SourceRepo          { return &pgSources{p} }
func (p *Postgres) Candidates() CandidateRepo    { return &pgCandidates{p} }
func (p *Postgres) Capabilities() CapabilityRepo { return &pgCapabilities{p} }
func (p *Postgres) Dependencies() DependencyRepo { return &pgDependencies{p} }
func (p *Postgres) Drafts() DraftRepo            { return &pgDrafts{p} }

func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }
func (p *Postgres) Close()                         { p.pool.Close() }

// translate converts driver errors into the store's sentinel errors so the API layer
// can map them onto status codes without importing pgx.
func translate(err error, what string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: %s", ErrNotFound, what)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return fmt.Errorf("%w: %s already exists", ErrConflict, what)
	}
	return err
}

// jsonb marshals a value for a jsonb column, substituting a valid empty document so
// NOT NULL columns are never handed a SQL NULL.
func jsonb(v any, empty string) ([]byte, error) {
	if v == nil {
		return []byte(empty), nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return []byte(empty), nil
	}
	return raw, nil
}

func jsonObject(v any) ([]byte, error) { return jsonb(v, "{}") }
func jsonArray(v any) ([]byte, error)  { return jsonb(v, "[]") }

func decodeJSON(raw []byte, dst any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

// TruncateAll clears every table. It exists for the integration tests, which need a
// known starting state, and is never called by the server.
func (p *Postgres) TruncateAll(ctx context.Context) error {
	_, err := p.pool.Exec(ctx, `
		TRUNCATE composition_components, business_agent_drafts, capability_dependencies,
		         capabilities, capability_candidates, external_sources
		RESTART IDENTITY CASCADE`)
	return err
}
