-- name: CreateOrg :one
INSERT INTO orgs (id, name, external_id, nats_public_key, nats_account_seed, billing_tier)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetOrg :one
SELECT * FROM orgs WHERE id = $1;

-- name: GetOrgByExternalID :one
SELECT * FROM orgs WHERE external_id = $1;

-- name: ListOrgs :many
SELECT * FROM orgs ORDER BY created_at ASC;

-- name: UpdateOrgBillingTier :one
UPDATE orgs SET billing_tier = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: UpdateOrgNatsPublicKey :exec
UPDATE orgs SET nats_public_key = $2, updated_at = now() WHERE id = $1;

-- name: UpdateOrgNatsAccountSeed :exec
UPDATE orgs SET nats_account_seed = $2, updated_at = now() WHERE id = $1;

-- name: DeleteOrg :exec
DELETE FROM orgs WHERE id = $1;
