-- +goose Up
-- Add org_id to events for multi-tenant isolation
ALTER TABLE events ADD COLUMN org_id VARCHAR(255);

-- Backfill org_id from api_keys for existing events
UPDATE events e
SET org_id = ak.org_id
FROM api_keys ak
WHERE e.api_key_id = ak.id AND ak.org_id IS NOT NULL;

-- Make org_id required for future events
ALTER TABLE events ALTER COLUMN org_id SET NOT NULL;

-- Drop environment column (no longer needed, handled via api_keys)
ALTER TABLE events DROP COLUMN IF EXISTS environment;

-- Index for org-scoped queries
CREATE INDEX idx_events_org_id ON events(org_id);
CREATE INDEX idx_events_org_id_created_at ON events(org_id, created_at DESC);
CREATE INDEX idx_events_org_id_topic ON events(org_id, topic);

-- +goose Down
DROP INDEX IF EXISTS idx_events_org_id_topic;
DROP INDEX IF EXISTS idx_events_org_id_created_at;
DROP INDEX IF EXISTS idx_events_org_id;
ALTER TABLE events ADD COLUMN environment VARCHAR(10) DEFAULT 'live';
ALTER TABLE events DROP COLUMN IF EXISTS org_id;
