-- Edges between registered capabilities, extracted from source bindings.
CREATE TABLE IF NOT EXISTS capability_dependencies (
    id                       TEXT PRIMARY KEY,
    capability_id            TEXT NOT NULL REFERENCES capabilities (id) ON DELETE CASCADE,
    depends_on_capability_id TEXT NOT NULL REFERENCES capabilities (id) ON DELETE CASCADE,
    dependency_type          TEXT NOT NULL CHECK (dependency_type IN
        ('uses_tool', 'uses_knowledge', 'uses_prompt', 'calls_skill', 'contains_step')),
    config_json              JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (capability_id <> depends_on_capability_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS capability_dependencies_edge_key
    ON capability_dependencies (capability_id, depends_on_capability_id, dependency_type);
CREATE INDEX IF NOT EXISTS capability_dependencies_capability_idx
    ON capability_dependencies (capability_id);
