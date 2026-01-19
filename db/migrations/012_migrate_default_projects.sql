-- +goose Up
-- Migrate existing data to default projects and make project_id NOT NULL

-- Create default project for each org that has API keys
INSERT INTO projects (id, org_id, name, slug, created_at, updated_at)
SELECT DISTINCT
    'prj_' || substr(md5(org_id || '_default'), 1, 27) as id,
    org_id,
    'Default' as name,
    'default' as slug,
    COALESCE(MIN(created_at), NOW()) as created_at,
    NOW() as updated_at
FROM api_keys
WHERE org_id IS NOT NULL
GROUP BY org_id
ON CONFLICT (org_id, slug) DO NOTHING;

-- Also create projects for orgs in other tables that might not have API keys yet
INSERT INTO projects (id, org_id, name, slug, created_at, updated_at)
SELECT DISTINCT
    'prj_' || substr(md5(org_id || '_default'), 1, 27) as id,
    org_id,
    'Default' as name,
    'default' as slug,
    NOW() as created_at,
    NOW() as updated_at
FROM webhooks
WHERE org_id IS NOT NULL AND org_id NOT IN (SELECT org_id FROM projects)
ON CONFLICT (org_id, slug) DO NOTHING;

INSERT INTO projects (id, org_id, name, slug, created_at, updated_at)
SELECT DISTINCT
    'prj_' || substr(md5(org_id || '_default'), 1, 27) as id,
    org_id,
    'Default' as name,
    'default' as slug,
    NOW() as created_at,
    NOW() as updated_at
FROM scheduled_events
WHERE org_id IS NOT NULL AND org_id NOT IN (SELECT org_id FROM projects)
ON CONFLICT (org_id, slug) DO NOTHING;

-- Update existing API keys to point to their org's default project
UPDATE api_keys SET project_id = p.id
FROM projects p
WHERE api_keys.org_id = p.org_id AND p.slug = 'default' AND api_keys.project_id IS NULL;

-- Update existing webhooks
UPDATE webhooks SET project_id = p.id
FROM projects p
WHERE webhooks.org_id = p.org_id AND p.slug = 'default' AND webhooks.project_id IS NULL;

-- Update existing scheduled_events
UPDATE scheduled_events SET project_id = p.id
FROM projects p
WHERE scheduled_events.org_id = p.org_id AND p.slug = 'default' AND scheduled_events.project_id IS NULL;

-- Update existing events (if they have org_id)
UPDATE events SET project_id = p.id
FROM projects p
WHERE events.org_id = p.org_id AND p.slug = 'default' AND events.project_id IS NULL;

-- Make project_id NOT NULL for api_keys (required for auth)
ALTER TABLE api_keys ALTER COLUMN project_id SET NOT NULL;

-- +goose Down
ALTER TABLE api_keys ALTER COLUMN project_id DROP NOT NULL;
