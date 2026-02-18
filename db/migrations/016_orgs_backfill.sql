-- +goose Up
-- Backfill existing org_id strings into the orgs table.
-- Uses a placeholder nats_public_key that will be replaced on first boot.

INSERT INTO orgs (id, name, nats_public_key, billing_tier)
SELECT DISTINCT org_id, org_id, 'pending_' || org_id, 'free'
FROM projects
WHERE org_id IS NOT NULL AND org_id != ''
ON CONFLICT (id) DO NOTHING;

-- Also backfill from api_keys
INSERT INTO orgs (id, name, nats_public_key, billing_tier)
SELECT DISTINCT org_id, org_id, 'pending_' || org_id, 'free'
FROM api_keys
WHERE org_id IS NOT NULL AND org_id != ''
ON CONFLICT (id) DO NOTHING;

-- +goose Down
-- No rollback for backfill — data was already there.
