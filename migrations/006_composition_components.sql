-- The authoritative composition of a draft: one row per participating capability.
CREATE TABLE IF NOT EXISTS composition_components (
    id             TEXT PRIMARY KEY,
    draft_id       TEXT NOT NULL REFERENCES business_agent_drafts (id) ON DELETE CASCADE,
    capability_id  TEXT NOT NULL REFERENCES capabilities (id) ON DELETE RESTRICT,
    component_role TEXT NOT NULL CHECK (component_role IN
        ('agents', 'workflows', 'skills', 'tools', 'knowledge_data', 'prompts', 'policies')),
    order_index    INTEGER NOT NULL DEFAULT 0,
    config_json    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS composition_components_draft_capability_key
    ON composition_components (draft_id, capability_id, component_role);
CREATE INDEX IF NOT EXISTS composition_components_draft_order_idx
    ON composition_components (draft_id, order_index);
