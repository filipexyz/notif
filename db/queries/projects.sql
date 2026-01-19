-- name: CreateProject :one
INSERT INTO projects (id, org_id, name, slug, created_at, updated_at)
VALUES ($1, $2, $3, $4, NOW(), NOW())
RETURNING *;

-- name: GetProject :one
SELECT * FROM projects WHERE id = $1;

-- name: GetProjectByOrgAndID :one
SELECT * FROM projects WHERE id = $1 AND org_id = $2;

-- name: GetProjectBySlug :one
SELECT * FROM projects WHERE org_id = $1 AND slug = $2;

-- name: ListProjectsByOrg :many
SELECT * FROM projects WHERE org_id = $1 ORDER BY created_at ASC;

-- name: UpdateProject :one
UPDATE projects
SET name = COALESCE(NULLIF($3, ''), name),
    slug = COALESCE(NULLIF($4, ''), slug),
    updated_at = NOW()
WHERE id = $1 AND org_id = $2
RETURNING *;

-- name: DeleteProject :exec
DELETE FROM projects WHERE id = $1 AND org_id = $2;

-- name: GetOrCreateDefaultProject :one
INSERT INTO projects (id, org_id, name, slug, created_at, updated_at)
VALUES ($1, $2, 'Default', 'default', NOW(), NOW())
ON CONFLICT (org_id, slug) DO UPDATE SET updated_at = NOW()
RETURNING *;

-- name: CountProjectsByOrg :one
SELECT COUNT(*) FROM projects WHERE org_id = $1;
