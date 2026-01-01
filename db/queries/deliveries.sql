-- name: CreateEventDelivery :one
INSERT INTO event_deliveries (event_id, receiver_type, receiver_id, consumer_name, client_id, status, attempt, delivered_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateEventDeliveryStatus :exec
UPDATE event_deliveries
SET status = $2, acked_at = $3, error = $4
WHERE id = $1;

-- name: UpdateEventDeliveryAcked :exec
UPDATE event_deliveries
SET status = 'acked', acked_at = NOW()
WHERE id = $1;

-- name: UpdateEventDeliveryNacked :exec
UPDATE event_deliveries
SET status = 'nacked', error = $2
WHERE id = $1;

-- name: UpdateEventDeliveryDLQ :exec
UPDATE event_deliveries
SET status = 'dlq', error = $2
WHERE id = $1;

-- name: GetEventDeliveries :many
SELECT * FROM event_deliveries
WHERE event_id = $1
ORDER BY created_at DESC;

-- name: GetEventDeliveriesWithWebhookURL :many
SELECT ed.*, w.url as webhook_url
FROM event_deliveries ed
LEFT JOIN webhooks w ON ed.receiver_type = 'webhook' AND ed.receiver_id = w.id
WHERE ed.event_id = $1
ORDER BY ed.created_at DESC;

-- name: GetDeliveriesByConsumer :many
SELECT * FROM event_deliveries
WHERE consumer_name = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: GetDeliveriesByWebhook :many
SELECT * FROM event_deliveries
WHERE receiver_type = 'webhook' AND receiver_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: CountDeliveriesByStatus :many
SELECT status, COUNT(*) as count
FROM event_deliveries
WHERE created_at > NOW() - INTERVAL '24 hours'
GROUP BY status;
