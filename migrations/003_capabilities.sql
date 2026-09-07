-- The capability registry: standardized, reusable units of business capability.
CREATE TABLE IF NOT EXISTS capabilities (
    id                 TEXT PRIMARY KEY,
    -- slug is the stable, human-readable identifier that composition YAML references.
    -- id carries a uniqueness suffix; slug deliberately does not.
    slug               TEXT NOT NULL,
    name               TEXT NOT NULL,
    type               TEXT NOT NULL CHECK (type IN
        ('agent', 'workflow', 'skill', 'tool', 'knowledge_data', 'prompt_template', 'policy')),
    subtype            TEXT NOT NULL DEFAULT '',
    description        TEXT NOT NULL DEFAULT '',
    source_system      TEXT NOT NULL DEFAULT '',
    external_id        TEXT NOT NULL DEFAULT '',
    business_domain    TEXT NOT NULL DEFAULT '',
    intents_json       JSONB NOT NULL DEFAULT '[]'::jsonb,
    tags_json          JSONB NOT NULL DEFAULT '[]'::jsonb,
    input_schema_json  JSONB NOT NULL DEFAULT '{}'::jsonb,
    output_schema_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    owner              TEXT NOT NULL DEFAULT '',
    permissions_json   JSONB NOT NULL DEFAULT '[]'::jsonb,
    risk_level         TEXT NOT NULL DEFAULT 'low' CHECK (risk_level IN ('low', 'medium', 'high')),
    reusable           BOOLEAN NOT NULL DEFAULT TRUE,
    status             TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deprecated')),
    metadata_json      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One capability per (type, slug). This is what stops the same CRM tool being
-- registered twice when it arrives from both a Dify DSL and a manual YAML.
CREATE UNIQUE INDEX IF NOT EXISTS capabilities_type_slug_key
    ON capabilities (type, slug);

CREATE INDEX IF NOT EXISTS capabilities_type_status_idx
    ON capabilities (type, status);
CREATE INDEX IF NOT EXISTS capabilities_external_idx
    ON capabilities (external_id) WHERE external_id <> '';

-- Recommendation matches on intents and tags, so both are GIN indexed.
CREATE INDEX IF NOT EXISTS capabilities_intents_gin
    ON capabilities USING GIN (intents_json jsonb_path_ops);
CREATE INDEX IF NOT EXISTS capabilities_tags_gin
    ON capabilities USING GIN (tags_json jsonb_path_ops);
