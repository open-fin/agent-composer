-- External systems that capabilities can be imported from.
CREATE TABLE IF NOT EXISTS external_sources (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL CHECK (type IN ('dify', 'mcp', 'data', 'manual')),
    base_url    TEXT NOT NULL DEFAULT '',
    auth_type   TEXT NOT NULL DEFAULT '',
    config_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Imports resolve a source by (name, type), creating it on first upload.
CREATE UNIQUE INDEX IF NOT EXISTS external_sources_name_type_key
    ON external_sources (name, type);
