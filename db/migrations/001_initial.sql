-- API Keys
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    key_prefix VARCHAR(32) NOT NULL,
    environment VARCHAR(10) NOT NULL CHECK (environment IN ('live', 'test')),
    name VARCHAR(255),
    rate_limit_per_second INT DEFAULT 100,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_environment ON api_keys(environment);

-- Events metadata (for dashboard, not primary storage - NATS is the source of truth)
CREATE TABLE events (
    id VARCHAR(32) PRIMARY KEY,
    topic VARCHAR(255) NOT NULL,
    api_key_id UUID REFERENCES api_keys(id),
    environment VARCHAR(10) NOT NULL,
    payload_size INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_events_topic ON events(topic);
CREATE INDEX idx_events_created_at ON events(created_at);
CREATE INDEX idx_events_api_key_id ON events(api_key_id);

-- Consumer groups metadata
CREATE TABLE consumer_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    api_key_id UUID REFERENCES api_keys(id),
    environment VARCHAR(10) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(name, api_key_id)
);
