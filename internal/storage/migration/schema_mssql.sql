-- Microsoft SQL Server / Azure SQL DDL Schema for ZITADEL Core
IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'zitadel_schema_version')
BEGIN
    CREATE TABLE zitadel_schema_version (
        version INT PRIMARY KEY,
        applied_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
        description NVARCHAR(255) NOT NULL
    );
END;

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'zitadel_instances')
BEGIN
    CREATE TABLE zitadel_instances (
        id NVARCHAR(64) PRIMARY KEY,
        name NVARCHAR(255) NOT NULL,
        default_domain NVARCHAR(255) NOT NULL,
        created_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
        updated_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET()
    );
END;

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'zitadel_orgs')
BEGIN
    CREATE TABLE zitadel_orgs (
        id NVARCHAR(64) PRIMARY KEY,
        instance_id NVARCHAR(64) NOT NULL FOREIGN KEY REFERENCES zitadel_instances(id) ON DELETE CASCADE,
        name NVARCHAR(255) NOT NULL,
        primary_domain NVARCHAR(255),
        created_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
        updated_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET()
    );
    CREATE INDEX idx_orgs_instance ON zitadel_orgs(instance_id);
END;

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'zitadel_projects')
BEGIN
    CREATE TABLE zitadel_projects (
        id NVARCHAR(64) PRIMARY KEY,
        instance_id NVARCHAR(64) NOT NULL FOREIGN KEY REFERENCES zitadel_instances(id) ON DELETE CASCADE,
        org_id NVARCHAR(64) NOT NULL FOREIGN KEY REFERENCES zitadel_orgs(id) ON DELETE NO ACTION,
        name NVARCHAR(255) NOT NULL,
        created_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
        updated_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET()
    );
    CREATE INDEX idx_projects_instance_org ON zitadel_projects(instance_id, org_id);
END;

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'zitadel_apps')
BEGIN
    CREATE TABLE zitadel_apps (
        id NVARCHAR(64) PRIMARY KEY,
        instance_id NVARCHAR(64) NOT NULL FOREIGN KEY REFERENCES zitadel_instances(id) ON DELETE CASCADE,
        project_id NVARCHAR(64) NOT NULL FOREIGN KEY REFERENCES zitadel_projects(id) ON DELETE NO ACTION,
        name NVARCHAR(255) NOT NULL,
        app_type NVARCHAR(32) NOT NULL,
        client_id NVARCHAR(128) UNIQUE,
        client_secret_hash NVARCHAR(255),
        redirect_uris NVARCHAR(MAX),
        post_logout_redirect_uris NVARCHAR(MAX),
        created_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
        updated_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET()
    );
    CREATE INDEX idx_apps_client_id ON zitadel_apps(client_id);
END;

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'zitadel_users')
BEGIN
    CREATE TABLE zitadel_users (
        id NVARCHAR(64) PRIMARY KEY,
        instance_id NVARCHAR(64) NOT NULL FOREIGN KEY REFERENCES zitadel_instances(id) ON DELETE CASCADE,
        org_id NVARCHAR(64) NOT NULL FOREIGN KEY REFERENCES zitadel_orgs(id) ON DELETE NO ACTION,
        username NVARCHAR(128) NOT NULL,
        email NVARCHAR(255),
        is_email_verified BIT DEFAULT 0,
        phone NVARCHAR(64),
        is_phone_verified BIT DEFAULT 0,
        first_name NVARCHAR(128),
        last_name NVARCHAR(128),
        display_name NVARCHAR(255),
        nick_name NVARCHAR(128),
        preferred_language NVARCHAR(16),
        gender NVARCHAR(32),
        user_type NVARCHAR(32) NOT NULL DEFAULT 'human',
        state NVARCHAR(32) NOT NULL DEFAULT 'active',
        password_hash NVARCHAR(255),
        metadata NVARCHAR(MAX),
        created_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
        updated_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
        CONSTRAINT uq_instance_org_username UNIQUE (instance_id, org_id, username)
    );
    CREATE INDEX idx_users_instance_org ON zitadel_users(instance_id, org_id);
    CREATE INDEX idx_users_email ON zitadel_users(instance_id, email);
END;

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'zitadel_credentials')
BEGIN
    CREATE TABLE zitadel_credentials (
        id NVARCHAR(64) PRIMARY KEY,
        user_id NVARCHAR(64) NOT NULL FOREIGN KEY REFERENCES zitadel_users(id) ON DELETE CASCADE,
        instance_id NVARCHAR(64) NOT NULL,
        cred_type NVARCHAR(32) NOT NULL,
        secret_hash NVARCHAR(MAX) NOT NULL,
        metadata NVARCHAR(MAX),
        created_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
        updated_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET()
    );
    CREATE INDEX idx_creds_user_type ON zitadel_credentials(user_id, cred_type);
END;

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'zitadel_roles')
BEGIN
    CREATE TABLE zitadel_roles (
        id NVARCHAR(64) PRIMARY KEY,
        instance_id NVARCHAR(64) NOT NULL FOREIGN KEY REFERENCES zitadel_instances(id) ON DELETE CASCADE,
        project_id NVARCHAR(64) NOT NULL FOREIGN KEY REFERENCES zitadel_projects(id) ON DELETE NO ACTION,
        role_key NVARCHAR(64) NOT NULL,
        display_name NVARCHAR(255),
        group_name NVARCHAR(128),
        created_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
        CONSTRAINT uq_project_role UNIQUE (project_id, role_key)
    );
END;

IF NOT EXISTS (SELECT * FROM sys.tables WHERE name = 'zitadel_policies')
BEGIN
    CREATE TABLE zitadel_policies (
        id NVARCHAR(64) PRIMARY KEY,
        instance_id NVARCHAR(64) NOT NULL,
        org_id NVARCHAR(64),
        policy_type NVARCHAR(64) NOT NULL,
        configuration NVARCHAR(MAX) NOT NULL,
        is_default BIT DEFAULT 0,
        created_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET(),
        updated_at DATETIMEOFFSET DEFAULT SYSDATETIMEOFFSET()
    );
    CREATE INDEX idx_policies_lookup ON zitadel_policies(instance_id, policy_type);
END;
