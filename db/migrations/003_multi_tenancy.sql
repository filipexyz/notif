-- +goose Up
-- Add organization support for multi-tenancy with Clerk
-- Each API key belongs to a Clerk organization (org_id format: org_XXXXXXXXX)

ALTER TABLE api_keys ADD COLUMN org_id VARCHAR(64);

CREATE INDEX idx_api_keys_org_id ON api_keys(org_id);
CREATE INDEX idx_api_keys_org_env ON api_keys(org_id, environment);

-- +goose Down
DROP INDEX IF EXISTS idx_api_keys_org_env;
DROP INDEX IF EXISTS idx_api_keys_org_id;
ALTER TABLE api_keys DROP COLUMN IF EXISTS org_id;
