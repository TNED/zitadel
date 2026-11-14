-- MySQL 8.0+ and MySQL HeatWave DDL Schema for ZITADEL Core
CREATE TABLE IF NOT EXISTS zitadel_schema_version (
    version INT PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    description VARCHAR(255) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS zitadel_instances (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    default_domain VARCHAR(255) NOT NULL,
    created_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS zitadel_orgs (
    id VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    primary_domain VARCHAR(255),
    created_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    INDEX idx_orgs_instance (instance_id),
    CONSTRAINT fk_orgs_instance FOREIGN KEY (instance_id) REFERENCES zitadel_instances(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS zitadel_projects (
    id VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL,
    org_id VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    INDEX idx_projects_instance_org (instance_id, org_id),
    CONSTRAINT fk_projects_instance FOREIGN KEY (instance_id) REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    CONSTRAINT fk_projects_org FOREIGN KEY (org_id) REFERENCES zitadel_orgs(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS zitadel_apps (
    id VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL,
    project_id VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    app_type VARCHAR(32) NOT NULL,
    client_id VARCHAR(128) UNIQUE,
    client_secret_hash VARCHAR(255),
    redirect_uris TEXT,
    post_logout_redirect_uris TEXT,
    created_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    INDEX idx_apps_client_id (client_id),
    CONSTRAINT fk_apps_instance FOREIGN KEY (instance_id) REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    CONSTRAINT fk_apps_project FOREIGN KEY (project_id) REFERENCES zitadel_projects(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS zitadel_users (
    id VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL,
    org_id VARCHAR(64) NOT NULL,
    username VARCHAR(128) NOT NULL,
    email VARCHAR(255),
    is_email_verified TINYINT(1) DEFAULT 0,
    phone VARCHAR(64),
    is_phone_verified TINYINT(1) DEFAULT 0,
    first_name VARCHAR(128),
    last_name VARCHAR(128),
    display_name VARCHAR(255),
    nick_name VARCHAR(128),
    preferred_language VARCHAR(16),
    gender VARCHAR(32),
    user_type VARCHAR(32) NOT NULL DEFAULT 'human',
    state VARCHAR(32) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    INDEX idx_users_instance_org (instance_id, org_id),
    INDEX idx_users_email (instance_id, email),
    CONSTRAINT uq_instance_org_username UNIQUE (instance_id, org_id, username),
    CONSTRAINT fk_users_instance FOREIGN KEY (instance_id) REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    CONSTRAINT fk_users_org FOREIGN KEY (org_id) REFERENCES zitadel_orgs(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS zitadel_credentials (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    instance_id VARCHAR(64) NOT NULL,
    cred_type VARCHAR(32) NOT NULL,
    secret_hash TEXT NOT NULL,
    metadata JSON,
    created_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    INDEX idx_creds_user_type (user_id, cred_type),
    CONSTRAINT fk_creds_user FOREIGN KEY (user_id) REFERENCES zitadel_users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS zitadel_roles (
    id VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL,
    project_id VARCHAR(64) NOT NULL,
    role_key VARCHAR(64) NOT NULL,
    display_name VARCHAR(255),
    group_name VARCHAR(128),
    created_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6),
    CONSTRAINT uq_project_role UNIQUE (project_id, role_key),
    CONSTRAINT fk_roles_instance FOREIGN KEY (instance_id) REFERENCES zitadel_instances(id) ON DELETE CASCADE,
    CONSTRAINT fk_roles_project FOREIGN KEY (project_id) REFERENCES zitadel_projects(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS zitadel_policies (
    id VARCHAR(64) PRIMARY KEY,
    instance_id VARCHAR(64) NOT NULL,
    org_id VARCHAR(64),
    policy_type VARCHAR(64) NOT NULL,
    configuration JSON NOT NULL,
    is_default TINYINT(1) DEFAULT 0,
    created_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    INDEX idx_policies_lookup (instance_id, policy_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
