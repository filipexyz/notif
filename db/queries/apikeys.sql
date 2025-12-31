-- name: GetAPIKeyByHash :one
SELECT id, key_prefix, environment, name, rate_limit_per_second, revoked_at, created_at
FROM api_keys
WHERE key_hash = $1 AND revoked_at IS NULL;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE api_keys SET last_used_at = NOW() WHERE id = $1;

-- name: CreateAPIKey :one
INSERT INTO api_keys (key_hash, key_prefix, environment, name, rate_limit_per_second)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, key_prefix, environment, name, rate_limit_per_second, created_at;

-- name: RevokeAPIKey :exec
UPDATE api_keys SET revoked_at = NOW() WHERE id = $1;

-- name: ListAPIKeys :many
SELECT id, key_prefix, environment, name, rate_limit_per_second, created_at, last_used_at, revoked_at
FROM api_keys
ORDER BY created_at DESC;
