-- name: GetAPIKeyByHash :one
SELECT id, key_prefix, name, rate_limit_per_second, revoked_at, created_at, org_id
FROM api_keys
WHERE key_hash = $1 AND revoked_at IS NULL;

-- name: GetAPIKeyByID :one
SELECT id, key_prefix, name, rate_limit_per_second, revoked_at, created_at, org_id
FROM api_keys
WHERE id = $1;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE api_keys SET last_used_at = NOW() WHERE id = $1;

-- name: CreateAPIKey :one
INSERT INTO api_keys (key_hash, key_prefix, name, rate_limit_per_second, org_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, key_prefix, name, rate_limit_per_second, created_at, org_id;

-- name: RevokeAPIKey :exec
UPDATE api_keys SET revoked_at = NOW() WHERE id = $1;

-- name: ListAPIKeys :many
SELECT id, key_prefix, name, rate_limit_per_second, created_at, last_used_at, revoked_at, org_id
FROM api_keys
ORDER BY created_at DESC;

-- Organization-scoped queries for dashboard

-- name: ListAPIKeysByOrg :many
SELECT id, key_prefix, name, rate_limit_per_second, created_at, last_used_at, revoked_at
FROM api_keys
WHERE org_id = $1
ORDER BY created_at DESC;

-- name: RevokeAPIKeyByOrg :exec
UPDATE api_keys SET revoked_at = NOW()
WHERE id = $1 AND org_id = $2 AND revoked_at IS NULL;

-- name: GetAPIKeyByIdAndOrg :one
SELECT id, key_prefix, name, rate_limit_per_second, revoked_at, created_at, org_id
FROM api_keys
WHERE id = $1 AND org_id = $2 AND revoked_at IS NULL;
