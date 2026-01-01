-- +goose Up
-- Allow webhooks to be org-scoped (not tied to a specific API key)
-- Dashboard-created webhooks use org_id, API-key-created webhooks use api_key_id

ALTER TABLE webhooks ADD COLUMN org_id VARCHAR(64);
ALTER TABLE webhooks ALTER COLUMN api_key_id DROP NOT NULL;

CREATE INDEX idx_webhooks_org_id ON webhooks(org_id);

-- Backfill org_id for existing webhooks from their api_key
UPDATE webhooks w
SET org_id = a.org_id
FROM api_keys a
WHERE w.api_key_id = a.id AND a.org_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_webhooks_org_id;
ALTER TABLE webhooks DROP COLUMN IF EXISTS org_id;
ALTER TABLE webhooks ALTER COLUMN api_key_id SET NOT NULL;
