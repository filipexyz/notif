-- +goose Up
CREATE TABLE scheduled_events (
    id VARCHAR(32) PRIMARY KEY,
    org_id VARCHAR(255) NOT NULL,
    topic VARCHAR(255) NOT NULL,
    data JSONB NOT NULL,
    scheduled_for TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',

    api_key_id UUID REFERENCES api_keys(id),
    error TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    executed_at TIMESTAMPTZ
);

CREATE INDEX idx_scheduled_pending ON scheduled_events(scheduled_for)
    WHERE status = 'pending';
CREATE INDEX idx_scheduled_org ON scheduled_events(org_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS scheduled_events;
