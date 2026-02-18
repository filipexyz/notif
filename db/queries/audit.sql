-- name: InsertAuditLog :exec
INSERT INTO audit_log (actor, action, org_id, target, detail, ip_address)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListAuditLogs :many
SELECT id, timestamp, actor, action, org_id, target, detail, ip_address
FROM audit_log
WHERE
    (sqlc.narg('org_id')::VARCHAR IS NULL OR org_id = sqlc.narg('org_id'))
    AND (sqlc.narg('action')::TEXT IS NULL OR action = sqlc.narg('action'))
    AND (sqlc.narg('since')::TIMESTAMPTZ IS NULL OR timestamp >= sqlc.narg('since'))
ORDER BY id DESC
LIMIT sqlc.arg('limit');
