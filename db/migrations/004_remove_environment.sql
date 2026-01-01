-- +goose Up
-- Remove environment distinction - org_id provides isolation

ALTER TABLE api_keys DROP COLUMN environment;
ALTER TABLE events DROP COLUMN environment;
ALTER TABLE webhooks DROP COLUMN environment;

DROP INDEX IF EXISTS idx_api_keys_environment;
DROP INDEX IF EXISTS idx_api_keys_org_env;

-- +goose Down
ALTER TABLE api_keys ADD COLUMN environment VARCHAR(10) DEFAULT 'live';
ALTER TABLE events ADD COLUMN environment VARCHAR(10) DEFAULT 'live';
ALTER TABLE webhooks ADD COLUMN environment VARCHAR(10) DEFAULT 'live';

CREATE INDEX idx_api_keys_environment ON api_keys(environment);
CREATE INDEX idx_api_keys_org_env ON api_keys(org_id, environment);
