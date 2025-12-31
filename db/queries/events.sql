-- name: CreateEvent :exec
INSERT INTO events (id, topic, api_key_id, environment, payload_size, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetEvent :one
SELECT id, topic, api_key_id, environment, payload_size, created_at
FROM events
WHERE id = $1;

-- name: ListEventsByTopic :many
SELECT id, topic, api_key_id, environment, payload_size, created_at
FROM events
WHERE topic LIKE $1
ORDER BY created_at DESC
LIMIT $2;

-- name: CountEventsByAPIKey :one
SELECT COUNT(*) FROM events WHERE api_key_id = $1;
