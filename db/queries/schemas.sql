-- name: CreateSchema :one
INSERT INTO schemas (id, org_id, project_id, name, topic_pattern, description, tags)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetSchema :one
SELECT * FROM schemas WHERE id = $1;

-- name: GetSchemaByName :one
SELECT * FROM schemas WHERE project_id = $1 AND name = $2;

-- name: ListSchemas :many
SELECT * FROM schemas
WHERE project_id = $1
ORDER BY name ASC;

-- name: ListSchemasByTag :many
SELECT * FROM schemas
WHERE project_id = $1 AND $2 = ANY(tags)
ORDER BY name ASC;

-- name: UpdateSchema :one
UPDATE schemas
SET topic_pattern = $2, description = $3, tags = $4, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteSchema :exec
DELETE FROM schemas WHERE id = $1;

-- name: GetSchemaForTopic :one
SELECT s.* FROM schemas s
WHERE s.project_id = $1
  AND (sqlc.arg(topic)::text LIKE REPLACE(REPLACE(s.topic_pattern, '.', '\.'), '*', '%')
       OR s.topic_pattern = sqlc.arg(topic)::text)
ORDER BY LENGTH(s.topic_pattern) DESC
LIMIT 1;

-- name: CreateSchemaVersion :one
INSERT INTO schema_versions (id, schema_id, version, schema_json, validation_mode, on_invalid, compatibility, examples, fingerprint, is_latest, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetSchemaVersion :one
SELECT * FROM schema_versions WHERE id = $1;

-- name: GetSchemaVersionByVersion :one
SELECT * FROM schema_versions
WHERE schema_id = $1 AND version = $2;

-- name: GetLatestSchemaVersion :one
SELECT * FROM schema_versions
WHERE schema_id = $1 AND is_latest = true;

-- name: ListSchemaVersions :many
SELECT * FROM schema_versions
WHERE schema_id = $1
ORDER BY created_at DESC;

-- name: SetSchemaVersionLatest :exec
UPDATE schema_versions
SET is_latest = (id = $2)
WHERE schema_id = $1;

-- name: DeleteSchemaVersion :exec
DELETE FROM schema_versions WHERE id = $1;

-- name: CreateSchemaValidation :one
INSERT INTO schema_validations (id, org_id, project_id, event_id, schema_id, schema_version_id, topic, valid, errors)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetSchemaValidation :one
SELECT * FROM schema_validations WHERE id = $1;

-- name: ListSchemaValidations :many
SELECT * FROM schema_validations
WHERE project_id = $1
ORDER BY validated_at DESC
LIMIT $2 OFFSET $3;

-- name: ListSchemaValidationsBySchema :many
SELECT * FROM schema_validations
WHERE schema_id = $1
ORDER BY validated_at DESC
LIMIT $2 OFFSET $3;

-- name: GetValidationStats :one
SELECT
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE valid = true) as valid_count,
    COUNT(*) FILTER (WHERE valid = false) as invalid_count
FROM schema_validations
WHERE project_id = $1
  AND validated_at > NOW() - INTERVAL '24 hours';

-- name: GetValidationStatsBySchema :one
SELECT
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE valid = true) as valid_count,
    COUNT(*) FILTER (WHERE valid = false) as invalid_count
FROM schema_validations
WHERE schema_id = $1
  AND validated_at > NOW() - INTERVAL '24 hours';
