-- Relational Session and Audit tables (Used ONLY when NoSQL is NOT active)
CREATE TABLE IF NOT EXISTS zitadel_sessions (
    id VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL,
    user_id VARCHAR(64) NOT NULL,
    user_agent VARCHAR(512),
    ip_address VARCHAR(64),
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
    actor_id VARCHAR(64) NOT NULL,
    action VARCHAR(64) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    details TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_instance_actor ON zitadel_audit_logs(instance_id, actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_instance_resource ON zitadel_audit_logs(instance_id, resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_created ON zitadel_audit_logs(created_at);
