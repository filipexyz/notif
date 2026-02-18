-- +goose Up
-- Add foreign key constraints from projects and api_keys to orgs.

ALTER TABLE projects ADD CONSTRAINT fk_projects_org
    FOREIGN KEY (org_id) REFERENCES orgs(id) ON DELETE CASCADE;

ALTER TABLE api_keys ADD CONSTRAINT fk_api_keys_org
    FOREIGN KEY (org_id) REFERENCES orgs(id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS fk_api_keys_org;
ALTER TABLE projects DROP CONSTRAINT IF EXISTS fk_projects_org;
