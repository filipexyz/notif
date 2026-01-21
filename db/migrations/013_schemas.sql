-- +goose Up

-- Schema definitions
CREATE TABLE schemas (
    id VARCHAR(32) PRIMARY KEY,
    org_id VARCHAR(32) NOT NULL,
    project_id VARCHAR(32) NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    topic_pattern VARCHAR(255) NOT NULL,
    description TEXT,
    tags TEXT[],
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(project_id, name)
);

-- Schema versions (immutable once created)
CREATE TABLE schema_versions (
    id VARCHAR(32) PRIMARY KEY,
    schema_id VARCHAR(32) NOT NULL REFERENCES schemas(id) ON DELETE CASCADE,
    version VARCHAR(50) NOT NULL,
    schema_json JSONB NOT NULL,
    validation_mode VARCHAR(20) DEFAULT 'strict',
    on_invalid VARCHAR(20) DEFAULT 'reject',
    compatibility VARCHAR(20) DEFAULT 'backward',
    examples JSONB,
    fingerprint VARCHAR(64),
    is_latest BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by VARCHAR(32),
    UNIQUE(schema_id, version)
);

-- Validation logs (for debugging/analytics)
CREATE TABLE schema_validations (
    id VARCHAR(32) PRIMARY KEY,
    org_id VARCHAR(32) NOT NULL,
    project_id VARCHAR(32) NOT NULL,
    event_id VARCHAR(32),
    schema_id VARCHAR(32),
    schema_version_id VARCHAR(32),
    topic VARCHAR(255),
    valid BOOLEAN NOT NULL,
    errors JSONB,
    validated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_schemas_project ON schemas(project_id);
CREATE INDEX idx_schemas_topic ON schemas(project_id, topic_pattern);
CREATE INDEX idx_schema_versions_schema ON schema_versions(schema_id);
CREATE INDEX idx_schema_versions_latest ON schema_versions(schema_id) WHERE is_latest = true;
CREATE INDEX idx_schema_validations_event ON schema_validations(event_id);
CREATE INDEX idx_schema_validations_schema ON schema_validations(schema_id);
CREATE INDEX idx_schema_validations_project ON schema_validations(project_id, validated_at DESC);

-- +goose Down
DROP TABLE IF EXISTS schema_validations;
DROP TABLE IF EXISTS schema_versions;
DROP TABLE IF EXISTS schemas;
