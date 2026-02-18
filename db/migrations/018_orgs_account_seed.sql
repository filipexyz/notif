-- +goose Up
-- Persist account NKey seed so key pairs survive restarts.
-- Without this, a new key pair is generated on every boot,
-- invalidating existing user JWTs.
--
-- TODO: encrypt at rest — wrap the seed with an application-level
-- encryption key (e.g. AES-256-GCM via ENCRYPTION_KEY env var)
-- before storing and decrypt on Boot(). Until then, protect the
-- database at the infrastructure level (encrypted volumes, restricted access).

ALTER TABLE orgs ADD COLUMN nats_account_seed TEXT;

-- +goose Down
ALTER TABLE orgs DROP COLUMN IF EXISTS nats_account_seed;
