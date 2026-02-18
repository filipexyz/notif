-- +goose Up
-- Create the central orgs entity for multi-tenant NATS accounts.

CREATE TABLE orgs (
    id              VARCHAR(32) PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    external_id     VARCHAR(255),
    nats_public_key VARCHAR(128) NOT NULL UNIQUE,
    billing_tier    VARCHAR(32) DEFAULT 'free',
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_orgs_external_id ON orgs(external_id);
CREATE INDEX idx_orgs_billing_tier ON orgs(billing_tier);

-- +goose Down
DROP TABLE IF EXISTS orgs;
