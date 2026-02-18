-- +goose Up

CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    timestamp   TIMESTAMPTZ DEFAULT now(),
    actor       TEXT NOT NULL,
    action      TEXT NOT NULL,
    org_id      VARCHAR(32),
    target      TEXT,
    detail      JSONB,
    ip_address  INET
);

CREATE INDEX idx_audit_log_org_id ON audit_log(org_id);
CREATE INDEX idx_audit_log_action ON audit_log(action);
CREATE INDEX idx_audit_log_timestamp ON audit_log(timestamp);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
