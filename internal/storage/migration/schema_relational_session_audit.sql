-- Relational Session and Audit tables (Used ONLY when NoSQL is NOT active)
CREATE TABLE IF NOT EXISTS zitadel_sessions (
    id VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    user_agent VARCHAR(512),
    client_ip VARCHAR(64),
    data TEXT,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_sessions_instance_user ON zitadel_sessions(instance_id, user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON zitadel_sessions(expires_at);

CREATE TABLE IF NOT EXISTS zitadel_audit_logs (
    id VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    editor_user VARCHAR(64),
    payload TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_instance_agg ON zitadel_audit_logs(instance_id, aggregate_type, aggregate_id);
CREATE INDEX IF NOT EXISTS idx_audit_created ON zitadel_audit_logs(created_at);
