-- Add organization support for multi-tenancy with Clerk
-- Each API key belongs to a Clerk organization (org_id format: org_XXXXXXXXX)

-- Add org_id column to api_keys
ALTER TABLE api_keys ADD COLUMN org_id VARCHAR(64);

-- Index for listing API keys by organization
CREATE INDEX idx_api_keys_org_id ON api_keys(org_id);

-- Composite index for org + environment queries
CREATE INDEX idx_api_keys_org_env ON api_keys(org_id, environment);

-- Note: org_id is nullable initially to allow gradual migration of existing keys.
-- After all existing keys are migrated to organizations, enforce NOT NULL:
-- ALTER TABLE api_keys ALTER COLUMN org_id SET NOT NULL;
