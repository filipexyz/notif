-- name: CreateEvent :exec
INSERT INTO events (id, topic, api_key_id, org_id, payload_size, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetEvent :one
SELECT id, topic, api_key_id, org_id, payload_size, created_at
FROM events
WHERE id = $1;

-- name: GetEventByIDAndOrg :one
SELECT id, topic, api_key_id, org_id, payload_size, created_at
FROM events
WHERE id = $1 AND org_id = $2;

-- name: ListEventsByOrg :many
SELECT id, topic, api_key_id, org_id, payload_size, created_at
FROM events
WHERE org_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: ListEventsByOrgAndTopic :many
SELECT id, topic, api_key_id, org_id, payload_size, created_at
FROM events
WHERE org_id = $1 AND topic LIKE $2
ORDER BY created_at DESC
LIMIT $3;

-- name: ListEventsByTopicAndOrg :many
SELECT id, topic, api_key_id, org_id, payload_size, created_at
FROM events
WHERE topic LIKE $1 AND org_id = $2
ORDER BY created_at DESC
LIMIT $3;

-- name: CountEventsByOrg :one
SELECT COUNT(*) FROM events WHERE org_id = $1;

-- name: CountEventsByAPIKey :one
SELECT COUNT(*) FROM events WHERE api_key_id = $1;

-- name: GetEventStats :one
SELECT
    COUNT(*) as total,
    COUNT(CASE WHEN created_at > NOW() - INTERVAL '24 hours' THEN 1 END) as last_24h,
    COUNT(CASE WHEN created_at > NOW() - INTERVAL '1 hour' THEN 1 END) as last_hour
FROM events
WHERE org_id = $1;
