-- +goose Up
-- Add project hierarchy within organizations
-- Each org can have multiple projects, with all resources scoped to a specific project

CREATE TABLE projects (
    id VARCHAR(32) PRIMARY KEY,
    org_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, slug)
);

CREATE INDEX idx_projects_org_id ON projects(org_id);

-- +goose Down
DROP INDEX IF EXISTS idx_projects_org_id;
DROP TABLE IF EXISTS projects;
