-- +goose Up
-- Fix constraint: allow client_id as fallback identifier for websocket receivers
ALTER TABLE event_deliveries DROP CONSTRAINT valid_receiver;

ALTER TABLE event_deliveries ADD CONSTRAINT valid_receiver CHECK (
    (receiver_type = 'webhook' AND receiver_id IS NOT NULL) OR
    (receiver_type = 'websocket' AND (consumer_name IS NOT NULL OR client_id IS NOT NULL))
);

-- +goose Down
ALTER TABLE event_deliveries DROP CONSTRAINT valid_receiver;

ALTER TABLE event_deliveries ADD CONSTRAINT valid_receiver CHECK (
    (receiver_type = 'webhook' AND receiver_id IS NOT NULL) OR
    (receiver_type = 'websocket' AND consumer_name IS NOT NULL)
);
