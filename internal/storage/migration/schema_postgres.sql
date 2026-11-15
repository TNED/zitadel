-- PostgreSQL DDL Schema for ZITADEL Core
CREATE TABLE IF NOT EXISTS zitadel_schema_version (
    version INT PRIMARY KEY,
    applied_at TIMESTAMPTZ DEFAULT NOW(),
    description TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS zitadel_instances (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    default_domain VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS zitadel_orgs (
    id VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    primary_domain VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_orgs_instance ON zitadel_orgs(instance_id);

CREATE TABLE IF NOT EXISTS zitadel_projects (
    id VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    org_id VARCHAR(64) NOT NULL REFERENCES zitadel_orgs(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_projects_instance_org ON zitadel_projects(instance_id, org_id);

CREATE TABLE IF NOT EXISTS zitadel_apps (
    id VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    project_id VARCHAR(64) NOT NULL REFERENCES zitadel_projects(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    app_type VARCHAR(32) NOT NULL,
    client_id VARCHAR(128) UNIQUE,
    client_secret_hash VARCHAR(255),
    redirect_uris TEXT,
    post_logout_redirect_uris TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_apps_client_id ON zitadel_apps(client_id);

CREATE TABLE IF NOT EXISTS zitadel_users (
    id VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    org_id VARCHAR(64) NOT NULL REFERENCES zitadel_orgs(id) ON DELETE CASCADE,
    username VARCHAR(128) NOT NULL,
    email VARCHAR(255),
    is_email_verified BOOLEAN DEFAULT FALSE,
    phone VARCHAR(64),
    is_phone_verified BOOLEAN DEFAULT FALSE,
    first_name VARCHAR(128),
    last_name VARCHAR(128),
    display_name VARCHAR(255),
    nick_name VARCHAR(128),
    preferred_language VARCHAR(16),
    gender VARCHAR(32),
    user_type VARCHAR(32) NOT NULL DEFAULT 'human',
    state VARCHAR(32) NOT NULL DEFAULT 'active',
    password_hash VARCHAR(255),
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_instance_org_username UNIQUE(instance_id, org_id, username)
);
CREATE INDEX IF NOT EXISTS idx_users_instance_org ON zitadel_users(instance_id, org_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON zitadel_users(instance_id, email);

CREATE TABLE IF NOT EXISTS zitadel_credentials (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL REFERENCES zitadel_users(id) ON DELETE CASCADE,
    instance_id VARCHAR(64) NOT NULL,
    cred_type VARCHAR(32) NOT NULL,
    secret_hash TEXT NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_creds_user_type ON zitadel_credentials(user_id, cred_type);

CREATE TABLE IF NOT EXISTS zitadel_roles (
    id VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    project_id VARCHAR(64) NOT NULL REFERENCES zitadel_projects(id) ON DELETE CASCADE,
    role_key VARCHAR(64) NOT NULL,
    display_name VARCHAR(255),
    group_name VARCHAR(128),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT uq_project_role UNIQUE(project_id, role_key)
);

CREATE TABLE IF NOT EXISTS zitadel_policies (
    id VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL,
    org_id VARCHAR(64),
    policy_type VARCHAR(64) NOT NULL,
    configuration JSONB NOT NULL,
    is_default BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_policies_lookup ON zitadel_policies(instance_id, policy_type);
