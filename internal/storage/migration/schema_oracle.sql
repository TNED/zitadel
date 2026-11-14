-- Oracle Database 26ai / 23ai / Autonomous DB DDL Schema for ZITADEL Core
CREATE TABLE IF NOT EXISTS zitadel_schema_version (
    version NUMBER(10) PRIMARY KEY,
    applied_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    description VARCHAR2(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS zitadel_instances (
    id VARCHAR2(64) PRIMARY KEY,
    name VARCHAR2(255) NOT NULL,
    default_domain VARCHAR2(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP
);

CREATE TABLE IF NOT EXISTS zitadel_orgs (
    id VARCHAR2(64) PRIMARY KEY,
    instance_id VARCHAR2(64) NOT NULL,
    name VARCHAR2(255) NOT NULL,
    primary_domain VARCHAR2(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    CONSTRAINT fk_orgs_instance FOREIGN KEY (instance_id) REFERENCES zitadel_instances(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_orgs_instance ON zitadel_orgs(instance_id);

CREATE TABLE IF NOT EXISTS zitadel_projects (
    id VARCHAR2(64) PRIMARY KEY,
    instance_id VARCHAR2(64) NOT NULL,
    org_id VARCHAR2(64) NOT NULL,
    name VARCHAR2(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    CONSTRAINT fk_projects_instance FOREIGN KEY (instance_id) REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    CONSTRAINT fk_projects_org FOREIGN KEY (org_id) REFERENCES zitadel_orgs(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_projects_instance_org ON zitadel_projects(instance_id, org_id);

CREATE TABLE IF NOT EXISTS zitadel_apps (
    id VARCHAR2(64) PRIMARY KEY,
    instance_id VARCHAR2(64) NOT NULL,
    project_id VARCHAR2(64) NOT NULL,
    name VARCHAR2(255) NOT NULL,
    app_type VARCHAR2(32) NOT NULL,
    client_id VARCHAR2(128) UNIQUE,
    client_secret_hash VARCHAR2(255),
    redirect_uris CLOB,
    post_logout_redirect_uris CLOB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    CONSTRAINT fk_apps_instance FOREIGN KEY (instance_id) REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    CONSTRAINT fk_apps_project FOREIGN KEY (project_id) REFERENCES zitadel_projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_apps_client_id ON zitadel_apps(client_id);

CREATE TABLE IF NOT EXISTS zitadel_users (
    id VARCHAR2(64) PRIMARY KEY,
    instance_id VARCHAR2(64) NOT NULL,
    org_id VARCHAR2(64) NOT NULL,
    username VARCHAR2(128) NOT NULL,
    email VARCHAR2(255),
    is_email_verified NUMBER(1) DEFAULT 0,
    phone VARCHAR2(64),
    is_phone_verified NUMBER(1) DEFAULT 0,
    first_name VARCHAR2(128),
    last_name VARCHAR2(128),
    display_name VARCHAR2(255),
    nick_name VARCHAR2(128),
    preferred_language VARCHAR2(16),
    gender VARCHAR2(32),
    user_type VARCHAR2(32) DEFAULT 'human' NOT NULL,
    state VARCHAR2(32) DEFAULT 'active' NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    CONSTRAINT uq_instance_org_username UNIQUE (instance_id, org_id, username),
    CONSTRAINT fk_users_instance FOREIGN KEY (instance_id) REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    CONSTRAINT fk_users_org FOREIGN KEY (org_id) REFERENCES zitadel_orgs(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_users_instance_org ON zitadel_users(instance_id, org_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON zitadel_users(instance_id, email);

CREATE TABLE IF NOT EXISTS zitadel_credentials (
    id VARCHAR2(64) PRIMARY KEY,
    user_id VARCHAR2(64) NOT NULL,
    instance_id VARCHAR2(64) NOT NULL,
    cred_type VARCHAR2(32) NOT NULL,
    secret_hash VARCHAR2(255) NOT NULL,
    metadata JSON,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    CONSTRAINT fk_creds_user FOREIGN KEY (user_id) REFERENCES zitadel_users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_creds_user_type ON zitadel_credentials(user_id, cred_type);

CREATE TABLE IF NOT EXISTS zitadel_roles (
    id VARCHAR2(64) PRIMARY KEY,
    instance_id VARCHAR2(64) NOT NULL,
    project_id VARCHAR2(64) NOT NULL,
    role_key VARCHAR2(64) NOT NULL,
    display_name VARCHAR2(255),
    group_name VARCHAR2(128),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    CONSTRAINT uq_project_role UNIQUE (project_id, role_key),
    CONSTRAINT fk_roles_instance FOREIGN KEY (instance_id) REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    CONSTRAINT fk_roles_project FOREIGN KEY (project_id) REFERENCES zitadel_projects(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS zitadel_policies (
    id VARCHAR2(64) PRIMARY KEY,
    instance_id VARCHAR2(64) NOT NULL,
    org_id VARCHAR2(64),
    policy_type VARCHAR2(64) NOT NULL,
    configuration JSON NOT NULL,
    is_default NUMBER(1) DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_policies_lookup ON zitadel_policies(instance_id, policy_type);

-- Relational Session and Audit tables for Oracle 26ai / 23ai (Used when NoSQL is NOT active)
CREATE TABLE IF NOT EXISTS zitadel_sessions (
    id VARCHAR2(64) PRIMARY KEY,
    instance_id VARCHAR2(64) NOT NULL,
    user_id VARCHAR2(64) NOT NULL,
    user_agent VARCHAR2(512),
    ip_address VARCHAR2(64),
    data CLOB,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_sessions_inst_user ON zitadel_sessions(instance_id, user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON zitadel_sessions(expires_at);

CREATE TABLE IF NOT EXISTS zitadel_audit_logs (
    id VARCHAR2(64) PRIMARY KEY,
    instance_id VARCHAR2(64) NOT NULL,
    aggregate_type VARCHAR2(64) NOT NULL,
    aggregate_id VARCHAR2(64) NOT NULL,
    event_type VARCHAR2(64) NOT NULL,
    editor_user VARCHAR2(64),
    payload CLOB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT SYSTIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_inst_agg ON zitadel_audit_logs(instance_id, aggregate_type, aggregate_id);
CREATE INDEX IF NOT EXISTS idx_audit_created ON zitadel_audit_logs(created_at);
