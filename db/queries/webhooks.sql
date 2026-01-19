-- name: CreateWebhook :one
INSERT INTO webhooks (org_id, project_id, url, topics, secret)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetWebhook :one
SELECT * FROM webhooks WHERE id = $1;

-- name: GetWebhookByIdAndOrg :one
SELECT * FROM webhooks WHERE id = $1 AND org_id = $2;

-- name: GetWebhooksByAPIKey :many
SELECT * FROM webhooks
WHERE api_key_id = $1
ORDER BY created_at DESC;

-- name: GetWebhooksByOrg :many
SELECT * FROM webhooks
WHERE org_id = $1
ORDER BY created_at DESC;

-- name: GetWebhooksByProject :many
SELECT * FROM webhooks
WHERE org_id = $1 AND project_id = $2
ORDER BY created_at DESC;

-- name: GetEnabledWebhooksByOrg :many
SELECT * FROM webhooks
WHERE org_id = $1 AND enabled = true
ORDER BY created_at DESC;

-- name: GetEnabledWebhooksByProject :many
SELECT * FROM webhooks
WHERE org_id = $1 AND project_id = $2 AND enabled = true
ORDER BY created_at DESC;

-- name: GetEnabledWebhooks :many
SELECT * FROM webhooks
WHERE enabled = true
ORDER BY created_at;

-- name: UpdateWebhook :one
UPDATE webhooks
SET url = $2, topics = $3, enabled = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteWebhook :exec
DELETE FROM webhooks WHERE id = $1;

-- name: DeleteWebhookByOrg :exec
DELETE FROM webhooks WHERE id = $1 AND org_id = $2;

-- name: DeleteWebhookByProject :exec
DELETE FROM webhooks WHERE id = $1 AND org_id = $2 AND project_id = $3;

-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (webhook_id, event_id, topic, status)
VALUES ($1, $2, $3, 'pending')
RETURNING *;

-- name: UpdateWebhookDelivery :exec
UPDATE webhook_deliveries
SET status = $2, attempt = $3, response_status = $4, response_body = $5, error = $6, delivered_at = $7
WHERE id = $1;

-- name: GetPendingDeliveries :many
SELECT wd.*, w.url, w.secret
FROM webhook_deliveries wd
JOIN webhooks w ON w.id = wd.webhook_id
WHERE wd.status = 'pending' AND w.enabled = true
ORDER BY wd.created_at
LIMIT $1;

-- name: GetWebhookDeliveries :many
SELECT * FROM webhook_deliveries
WHERE webhook_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: GetDeliveriesByEventID :many
SELECT wd.*, w.url as webhook_url
FROM webhook_deliveries wd
JOIN webhooks w ON w.id = wd.webhook_id
WHERE wd.event_id = $1
ORDER BY wd.created_at DESC;
