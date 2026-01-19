-- +goose Up
-- Add project_id to all resource tables for project-level isolation

-- API Keys (project-scoped, one key = one project)
ALTER TABLE api_keys ADD COLUMN project_id VARCHAR(32) REFERENCES projects(id);

-- Events
ALTER TABLE events ADD COLUMN project_id VARCHAR(32);

-- Webhooks
ALTER TABLE webhooks ADD COLUMN project_id VARCHAR(32);

-- Scheduled Events
ALTER TABLE scheduled_events ADD COLUMN project_id VARCHAR(32);

-- Indexes for project filtering
CREATE INDEX idx_api_keys_project_id ON api_keys(project_id);
CREATE INDEX idx_events_project_id ON events(project_id);
CREATE INDEX idx_webhooks_project_id ON webhooks(project_id);
CREATE INDEX idx_scheduled_events_project_id ON scheduled_events(project_id);

-- +goose Down
DROP INDEX IF EXISTS idx_scheduled_events_project_id;
DROP INDEX IF EXISTS idx_webhooks_project_id;
DROP INDEX IF EXISTS idx_events_project_id;
DROP INDEX IF EXISTS idx_api_keys_project_id;

ALTER TABLE scheduled_events DROP COLUMN IF EXISTS project_id;
ALTER TABLE webhooks DROP COLUMN IF EXISTS project_id;
ALTER TABLE events DROP COLUMN IF EXISTS project_id;
ALTER TABLE api_keys DROP COLUMN IF EXISTS project_id;
