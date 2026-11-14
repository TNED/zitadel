-- SQLite DDL Schema for ZITADEL Core
CREATE TABLE IF NOT EXISTS zitadel_schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    description TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS zitadel_instances (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    default_domain TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS zitadel_orgs (
    id TEXT PRIMARY KEY,
    instance_id TEXT NOT NULL,
    name TEXT NOT NULL,
    primary_domain TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (instance_id) REFERENCES zitadel_instances(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_orgs_instance ON zitadel_orgs(instance_id);

CREATE TABLE IF NOT EXISTS zitadel_projects (
    id TEXT PRIMARY KEY,
    instance_id TEXT NOT NULL,
    org_id TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (instance_id) REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    FOREIGN KEY (org_id) REFERENCES zitadel_orgs(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_projects_instance_org ON zitadel_projects(instance_id, org_id);

CREATE TABLE IF NOT EXISTS zitadel_apps (
    id TEXT PRIMARY KEY,
    instance_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    name TEXT NOT NULL,
    app_type TEXT NOT NULL,
    client_id TEXT UNIQUE,
    client_secret_hash TEXT,
    redirect_uris TEXT,
    post_logout_redirect_uris TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (instance_id) REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES zitadel_projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_apps_client_id ON zitadel_apps(client_id);

CREATE TABLE IF NOT EXISTS zitadel_users (
    id TEXT PRIMARY KEY,
    instance_id TEXT NOT NULL,
    org_id TEXT NOT NULL,
    username TEXT NOT NULL,
    email TEXT,
    is_email_verified INTEGER DEFAULT 0,
    phone TEXT,
    is_phone_verified INTEGER DEFAULT 0,
    first_name TEXT,
    last_name TEXT,
    display_name TEXT,
    nick_name TEXT,
    preferred_language TEXT,
    gender TEXT,
    user_type TEXT NOT NULL DEFAULT 'human',
    state TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (instance_id) REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    FOREIGN KEY (org_id) REFERENCES zitadel_orgs(id) ON DELETE CASCADE,
    UNIQUE(instance_id, org_id, username)
);
CREATE INDEX IF NOT EXISTS idx_users_instance_org ON zitadel_users(instance_id, org_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON zitadel_users(instance_id, email);

CREATE TABLE IF NOT EXISTS zitadel_credentials (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    instance_id TEXT NOT NULL,
    cred_type TEXT NOT NULL,
    secret_hash TEXT NOT NULL,
    metadata TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES zitadel_users(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_creds_user_type ON zitadel_credentials(user_id, cred_type);

CREATE TABLE IF NOT EXISTS zitadel_roles (
    id TEXT PRIMARY KEY,
    instance_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    role_key TEXT NOT NULL,
    display_name TEXT,
    group_name TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (instance_id) REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    FOREIGN KEY (project_id) REFERENCES zitadel_projects(id) ON DELETE CASCADE,
    UNIQUE(project_id, role_key)
);

CREATE TABLE IF NOT EXISTS zitadel_policies (
    id TEXT PRIMARY KEY,
    instance_id TEXT NOT NULL,
    org_id TEXT,
    policy_type TEXT NOT NULL,
    configuration TEXT NOT NULL,
    is_default INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_policies_lookup ON zitadel_policies(instance_id, policy_type);
