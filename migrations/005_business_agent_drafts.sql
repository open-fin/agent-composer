-- Generated business agent drafts. Phase 1 never publishes or executes these.
CREATE TABLE IF NOT EXISTS business_agent_drafts (
    id               TEXT PRIMARY KEY,
    slug             TEXT NOT NULL,
    name             TEXT NOT NULL,
    goal             TEXT NOT NULL DEFAULT '',
    description      TEXT NOT NULL DEFAULT '',
    harness_type     TEXT NOT NULL CHECK (harness_type IN
        ('dify_workflow', 'jiuwen_swarm', 'http_agent', 'manual')),
    status           TEXT NOT NULL DEFAULT 'draft' CHECK (status IN
        ('draft', 'needs_review', 'ready_for_eval')),
    -- Denormalized snapshot of the recommendation. composition_components is the
    -- authoritative record of what the draft consists of; this column is regenerated
    -- from it on every write and exists so the plan can be replayed in the UI.
    composition_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    governance_json  JSONB NOT NULL DEFAULT '{}'::jsonb,
    -- Rendered artifact, also regenerated on every write.
    yaml_text        TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS business_agent_drafts_slug_key
    ON business_agent_drafts (slug);
CREATE INDEX IF NOT EXISTS business_agent_drafts_status_idx
    ON business_agent_drafts (status);
