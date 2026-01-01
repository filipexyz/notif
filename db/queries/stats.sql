-- Stats queries for observability dashboard

-- name: GetAPIKeyStats :one
SELECT
    COUNT(*) FILTER (WHERE revoked_at IS NULL) as total,
    COUNT(*) FILTER (WHERE revoked_at IS NULL AND last_used_at > NOW() - INTERVAL '24 hours') as active_24h
FROM api_keys
WHERE org_id = $1;

-- name: GetWebhookStats :one
SELECT
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE enabled = true) as enabled,
    COUNT(*) FILTER (WHERE enabled = false) as disabled
FROM webhooks w
JOIN api_keys ak ON ak.id = w.api_key_id
WHERE ak.org_id = $1 AND ak.revoked_at IS NULL;

-- name: GetWebhookDeliveryStats :one
SELECT
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE status = 'success') as success_count,
    COUNT(*) FILTER (WHERE status = 'failed') as failed_count,
    COUNT(*) FILTER (WHERE status = 'pending') as pending_count
FROM webhook_deliveries wd
JOIN webhooks w ON w.id = wd.webhook_id
JOIN api_keys ak ON ak.id = w.api_key_id
WHERE ak.org_id = $1 AND wd.created_at > NOW() - INTERVAL '24 hours';

-- name: GetWebhookDeliveryStatsByWebhook :many
SELECT
    w.id as webhook_id,
    w.url,
    COUNT(*) as total_deliveries,
    COUNT(*) FILTER (WHERE wd.status = 'success') as success_count,
    AVG(EXTRACT(EPOCH FROM (wd.delivered_at - wd.created_at)) * 1000)::int as avg_latency_ms
FROM webhooks w
JOIN api_keys ak ON ak.id = w.api_key_id
LEFT JOIN webhook_deliveries wd ON w.id = wd.webhook_id AND wd.created_at > NOW() - INTERVAL '24 hours'
WHERE ak.org_id = $1 AND ak.revoked_at IS NULL
GROUP BY w.id, w.url;
