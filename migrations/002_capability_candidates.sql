-- Extracted, not-yet-registered capabilities awaiting human review.
CREATE TABLE IF NOT EXISTS capability_candidates (
    id               TEXT PRIMARY KEY,
    source_id        TEXT NOT NULL REFERENCES external_sources (id) ON DELETE CASCADE,
    source_system    TEXT NOT NULL,
    external_id      TEXT NOT NULL,
    candidate_type   TEXT NOT NULL CHECK (candidate_type IN
        ('agent', 'workflow', 'skill', 'tool', 'knowledge_data', 'prompt_template', 'policy')),
    name             TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    raw_payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    extracted_json   JSONB NOT NULL DEFAULT '{}'::jsonb,
    inferred_json    JSONB NOT NULL DEFAULT '{}'::jsonb,
    review_json      JSONB NOT NULL DEFAULT '{}'::jsonb,
    status           TEXT NOT NULL CHECK (status IN
        ('extracted', 'inferred', 'needs_review', 'registered', 'rejected')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The natural key of a candidate. Re-importing the same DSL refreshes rows in place
-- instead of duplicating every capability on each upload.
CREATE UNIQUE INDEX IF NOT EXISTS capability_candidates_source_external_key
    ON capability_candidates (source_id, external_id);

CREATE INDEX IF NOT EXISTS capability_candidates_status_idx
    ON capability_candidates (status);
CREATE INDEX IF NOT EXISTS capability_candidates_type_idx
    ON capability_candidates (candidate_type);
