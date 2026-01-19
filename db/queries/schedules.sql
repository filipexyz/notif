-- name: CreateScheduledEvent :one
INSERT INTO scheduled_events (id, org_id, project_id, topic, data, scheduled_for, api_key_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetScheduledEvent :one
SELECT * FROM scheduled_events WHERE id = $1 AND org_id = $2;

-- name: GetScheduledEventByProject :one
SELECT * FROM scheduled_events WHERE id = $1 AND org_id = $2 AND project_id = $3;

-- name: ListScheduledEvents :many
SELECT * FROM scheduled_events
WHERE org_id = $1
ORDER BY scheduled_for DESC
LIMIT $2 OFFSET $3;

-- name: ListScheduledEventsByProject :many
SELECT * FROM scheduled_events
WHERE org_id = $1 AND project_id = $2
ORDER BY scheduled_for DESC
LIMIT $3 OFFSET $4;

-- name: ListScheduledEventsByStatus :many
SELECT * FROM scheduled_events
WHERE org_id = $1 AND status = $2
ORDER BY scheduled_for DESC
LIMIT $3 OFFSET $4;

-- name: ListScheduledEventsByProjectAndStatus :many
SELECT * FROM scheduled_events
WHERE org_id = $1 AND project_id = $2 AND status = $3
ORDER BY scheduled_for DESC
LIMIT $4 OFFSET $5;

-- name: GetPendingScheduledEvents :many
SELECT * FROM scheduled_events
WHERE scheduled_for <= NOW() AND status = 'pending'
ORDER BY scheduled_for ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: GetScheduledEventForExecution :one
SELECT * FROM scheduled_events
WHERE id = $1 AND org_id = $2 AND status = 'pending'
FOR UPDATE SKIP LOCKED;

-- name: UpdateScheduledEventStatus :exec
UPDATE scheduled_events
SET status = sqlc.arg(status)::text,
    executed_at = CASE WHEN sqlc.arg(status)::text = 'completed' THEN NOW() ELSE executed_at END,
    error = sqlc.arg(error)
WHERE id = sqlc.arg(id);

-- name: CancelScheduledEvent :execrows
UPDATE scheduled_events
SET status = 'cancelled'
WHERE id = $1 AND org_id = $2 AND status = 'pending';

-- name: CancelScheduledEventByProject :execrows
UPDATE scheduled_events
SET status = 'cancelled'
WHERE id = $1 AND org_id = $2 AND project_id = $3 AND status = 'pending';

-- name: CountScheduledEventsByStatus :one
SELECT
    COUNT(*) FILTER (WHERE status = 'pending') as pending,
    COUNT(*) FILTER (WHERE status = 'completed') as completed,
    COUNT(*) FILTER (WHERE status = 'cancelled') as cancelled,
    COUNT(*) FILTER (WHERE status = 'failed') as failed
FROM scheduled_events
WHERE org_id = $1;

-- name: CountScheduledEventsByProjectStatus :one
SELECT
    COUNT(*) FILTER (WHERE status = 'pending') as pending,
    COUNT(*) FILTER (WHERE status = 'completed') as completed,
    COUNT(*) FILTER (WHERE status = 'cancelled') as cancelled,
    COUNT(*) FILTER (WHERE status = 'failed') as failed
FROM scheduled_events
WHERE org_id = $1 AND project_id = $2;
