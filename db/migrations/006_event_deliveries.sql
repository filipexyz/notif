-- +goose Up
-- Unified event delivery tracking
-- Tracks ALL deliveries (webhook and websocket) for observability

CREATE TABLE event_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id VARCHAR(64) NOT NULL,

    -- Receiver info
    receiver_type VARCHAR(16) NOT NULL,  -- 'webhook' or 'websocket'
    receiver_id UUID,                     -- webhook_id for webhooks, NULL for websocket
    consumer_name VARCHAR(128),           -- NATS consumer name for websocket
    client_id VARCHAR(64),                -- WebSocket client identifier

    -- Delivery state
    status VARCHAR(16) NOT NULL DEFAULT 'pending',  -- pending, delivered, acked, nacked, dlq
    attempt INT NOT NULL DEFAULT 1,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ,             -- When sent to receiver
    acked_at TIMESTAMPTZ,                 -- When ACK received

    -- Error info (for nacked/dlq)
    error TEXT,

    -- Ensure proper receiver info based on type
    CONSTRAINT valid_receiver CHECK (
        (receiver_type = 'webhook' AND receiver_id IS NOT NULL) OR
        (receiver_type = 'websocket' AND consumer_name IS NOT NULL)
    )
);

-- Query by event
CREATE INDEX idx_event_deliveries_event_id ON event_deliveries(event_id);

-- Query by receiver
CREATE INDEX idx_event_deliveries_receiver ON event_deliveries(receiver_type, receiver_id);

-- Query by status
CREATE INDEX idx_event_deliveries_status ON event_deliveries(status);

-- Query by consumer (partial index for websocket only)
CREATE INDEX idx_event_deliveries_consumer ON event_deliveries(consumer_name) WHERE consumer_name IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS event_deliveries;
