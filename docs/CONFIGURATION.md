# ZITADEL Headless IAM Configuration Specification

ZITADEL is a headless Identity and Access Management (IAM) engine. It provides gRPC, ConnectRPC, REST, OIDC/OAuth 2.0, SAML 2.0, and SCIM 2.0 APIs without any built-in frontend UI pages or web assets.

This specification documents the configuration options for ZITADEL's multi-database persistence layer, including **7 Relational Database engines**, **2 Multi-Cloud NoSQL engines**, deployment modes, and automated migration & bootstrapping.

---

## 1. Storage Architecture & Deployment Modes

ZITADEL supports two distinct persistence operating modes controlled via Go build tags:

```
┌────────────────────────────────────────────────────────────────────────┐
│                        ZITADEL Headless Core                           │
│  (Users, Orgs, Projects, Apps, Credentials, Roles, Policies, AuthZ)   │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
           ┌────────────────────────┴────────────────────────┐
           ▼                                                 ▼
┌──────────────────────────────┐          ┌──────────────────────────────────────┐
│     Pure Relational Mode     │          │         Hybrid Storage Mode          │
│  (!storage_dynamodb &&       │          │  (storage_dynamodb ||                │
│   !storage_firestore)        │          │   storage_firestore)                 │
├──────────────────────────────┤          ├──────────────────────────────────────┤
│ Core Entities : Relational   │          │ Core Entities : Relational (ACID)    │
│ Sessions      : Relational   │          │ Sessions      : Multi-Cloud NoSQL    │
│                 (Janitor GC) │          │                 (Hardware TTL)       │
│ Audit Logs    : Relational   │          │ Audit Logs    : Multi-Cloud NoSQL    │
│                 (Async Batch)│          │                 (Append-Only Stream) │
│ No NoSQL SDKs or code loaded │          │ Strict Isolation: zero fallback code │
└──────────────────────────────┘          └──────────────────────────────────────┘
```

### 1.1 Pure Relational Mode (Default)
When built without NoSQL build tags, ZITADEL manages all domain entities, user sessions, and audit logs within your chosen relational database:
* **Sessions**: Stored in `zitadel_sessions` with query-time expiration filters and a lightweight background `JanitorWorker` garbage collector.
* **Audit Logs**: Buffered in memory and flushed asynchronously in batches (`BufferedBatcher`) to `zitadel_audit_logs`.
* **Zero Overhead**: AWS SDK v2 and GCP Firestore SDK are completely excluded at compile time.

### 1.2 Hybrid Storage Mode (Strict Isolation)
When compiled with `storage_dynamodb` or `storage_firestore`:
* **Core Entities**: Persisted in the relational database (ACID compliance, referential integrity).
* **Sessions**: Offloaded to DynamoDB or Firestore with native hardware TTL policies.
* **Audit Logs**: Streamed directly to NoSQL collections/tables.
* **Strict Isolation**: All relational session/audit tables, queries, and background Janitor routines are compiled out. The relational database contains **zero session or audit tables**.

---

## 2. Relational Database Configuration

ZITADEL supports 7 enterprise relational database engines configured under the `Storage.Relational` section in YAML or via environment variables.

### 2.1 SQLite
* **Build Tag**: `storage_sqlite`
* **Driver**: `modernc.org/sqlite` (Pure Go, 100% Zero CGO, cross-platform)
* **Best For**: Embedded instances, edge appliances, CI/CD automated testing, local development.

```yaml
Storage:
  Relational:
    Dialect: "sqlite"
    DSN: "file:/var/lib/zitadel/data.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
    MaxOpenConns: 1
    MaxIdleConns: 1
    ConnMaxLifetime: "0s"
```
*Environment Variable*:
```bash
ZITADEL_STORAGE_RELATIONAL_DIALECT=sqlite
ZITADEL_STORAGE_RELATIONAL_DSN=file:/var/lib/zitadel/data.db?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)
```

---

### 2.2 MySQL 8.0+ & Oracle MySQL HeatWave
* **Build Tag**: `storage_mysql`
* **Driver**: `github.com/go-sql-driver/mysql`
* **Best For**: MySQL 8.0+ deployments, AWS RDS MySQL, Google Cloud SQL MySQL, Oracle Cloud MySQL HeatWave.

```yaml
Storage:
  Relational:
    Dialect: "mysql"
    DSN: "zitadel_user:SecretPassword123!@tcp(mysql.internal:3306)/zitadel?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&tls=preferred"
    MaxOpenConns: 50
    MaxIdleConns: 10
    ConnMaxLifetime: "30m"
```
*Environment Variables*:
```bash
ZITADEL_STORAGE_RELATIONAL_DIALECT=mysql
ZITADEL_STORAGE_RELATIONAL_DSN="zitadel_user:SecretPassword123!@tcp(mysql.internal:3306)/zitadel?parseTime=true&charset=utf8mb4"
```

---

### 2.3 PostgreSQL
* **Build Tag**: `storage_postgres`
* **Driver**: `github.com/jackc/pgx/v5`
* **Best For**: PostgreSQL 14+, AWS RDS PostgreSQL, Supabase, Azure Database for PostgreSQL.

```yaml
Storage:
  Relational:
    Dialect: "postgres"
    DSN: "postgres://zitadel_user:SecretPassword123!@postgres.internal:5432/zitadel?sslmode=verify-full&sslrootcert=/certs/root.crt"
    MaxOpenConns: 50
    MaxIdleConns: 10
    ConnMaxLifetime: "30m"
```
*Environment Variables*:
```bash
ZITADEL_STORAGE_RELATIONAL_DIALECT=postgres
ZITADEL_STORAGE_RELATIONAL_DSN="postgres://zitadel_user:SecretPassword123!@postgres.internal:5432/zitadel?sslmode=disable"
```

---

### 2.4 Amazon Aurora DSQL
* **Build Tag**: `storage_dsql`
* **Driver**: `github.com/jackc/pgx/v5` (PostgreSQL protocol with AWS IAM token auth)
* **Best For**: Multi-Region active-active serverless PostgreSQL workloads on AWS.

```yaml
Storage:
  Relational:
    Dialect: "dsql"
    DSN: "postgres://admin@a1b2c3d4e5.dsql.us-east-1.on.aws:5432/postgres?sslmode=require"
    MaxOpenConns: 100
    MaxIdleConns: 20
    ConnMaxLifetime: "15m"
    DSQL:
      Region: "us-east-1"
      MaxRetries: 5 # Automatic OCC (Optimistic Concurrency Control) retry handler
```
*Environment Variables*:
```bash
ZITADEL_STORAGE_RELATIONAL_DIALECT=dsql
ZITADEL_STORAGE_RELATIONAL_DSN="postgres://admin@a1b2c3d4e5.dsql.us-east-1.on.aws:5432/postgres?sslmode=require"
AWS_REGION=us-east-1
```

---

### 2.5 Microsoft SQL Server & Azure SQL Database
* **Build Tag**: `storage_mssql`
* **Driver**: `github.com/microsoft/go-mssqldb`
* **Best For**: Enterprise Microsoft ecosystems, Azure SQL Database, SQL Server 2019/2022.

```yaml
Storage:
  Relational:
    Dialect: "mssql"
    DSN: "sqlserver://zitadel_user:SecretPassword123!@sqlserver.internal:1433?database=zitadel&encrypt=true&TrustServerCertificate=false"
    MaxOpenConns: 50
    MaxIdleConns: 10
    ConnMaxLifetime: "30m"
```
*Environment Variables*:
```bash
ZITADEL_STORAGE_RELATIONAL_DIALECT=mssql
ZITADEL_STORAGE_RELATIONAL_DSN="sqlserver://zitadel_user:SecretPassword123!@sqlserver.internal:1433?database=zitadel&encrypt=true"
```

---

### 2.6 Oracle Database 23ai & Oracle Autonomous Database
* **Build Tag**: `storage_oracle`
* **Driver**: `github.com/sijms/go-ora/v2` (Pure Go, Zero Oracle Client/OCI binaries required)
* **Best For**: Oracle Cloud Infrastructure (OCI) Autonomous Database, Oracle Database 23ai.

```yaml
Storage:
  Relational:
    Dialect: "oracle"
    DSN: "oracle://zitadel_user:SecretPassword123!@oracle.internal:1521/FREEPDB1?SSL=enable&WALLET=/etc/oracle/wallet"
    MaxOpenConns: 30
    MaxIdleConns: 5
    ConnMaxLifetime: "30m"
```
*Environment Variables*:
```bash
ZITADEL_STORAGE_RELATIONAL_DIALECT=oracle
ZITADEL_STORAGE_RELATIONAL_DSN="oracle://zitadel_user:SecretPassword123!@oracle.internal:1521/FREEPDB1"
```

---

## 3. Multi-Cloud NoSQL Configuration (Optional)

When compiled with `storage_dynamodb` or `storage_firestore`, configure the `Storage.NoSQL` block:

### 3.1 Amazon DynamoDB
* **Build Tag**: `storage_dynamodb`
* **SDK**: AWS SDK for Go v2
* **Features**: Automatic partition key routing, serverless on-demand scaling, native TTL via Unix timestamp on `expires_at`.

```yaml
Storage:
  NoSQL:
    Provider: "dynamodb"
    DynamoDB:
      Region: "us-east-1"
      Endpoint: "" # Leave empty for AWS; set to http://localhost:8000 for localstack
      SessionTableName: "zitadel_sessions"
      AuditTableName: "zitadel_audit_logs"
```
*Environment Variables*:
```bash
ZITADEL_STORAGE_NOSQL_PROVIDER=dynamodb
AWS_REGION=us-east-1
ZITADEL_STORAGE_NOSQL_DYNAMODB_SESSIONTABLENAME=zitadel_sessions
ZITADEL_STORAGE_NOSQL_DYNAMODB_AUDITTABLENAME=zitadel_audit_logs
```

---

### 3.2 Google Cloud Firestore
* **Build Tag**: `storage_firestore`
* **SDK**: `cloud.google.com/go/firestore`
* **Features**: Multi-region replication, native TTL policy on `expires_at`, document collections.

```yaml
Storage:
  NoSQL:
    Provider: "firestore"
    Firestore:
      ProjectID: "my-gcp-project"
      DatabaseID: "(default)"
      SessionCollection: "zitadel_sessions"
      AuditCollection: "zitadel_audit_logs"
```
*Environment Variables*:
```bash
ZITADEL_STORAGE_NOSQL_PROVIDER=firestore
GOOGLE_CLOUD_PROJECT=my-gcp-project
ZITADEL_STORAGE_NOSQL_FIRESTORE_SESSIONCOLLECTION=zitadel_sessions
ZITADEL_STORAGE_NOSQL_FIRESTORE_AUDITCOLLECTION=zitadel_audit_logs
```

---

## 4. Automated Database Migration & Declarative Bootstrapping

ZITADEL eliminates manual CockroachDB/Postgres multi-step setup scripts in favor of **native dialect DDL execution** and **declarative bootstrapping**.

### 4.1 Auto-Migration on Startup
To automatically verify, create missing DDL tables, and seed the initial instance upon process startup, pass the `--auto-migrate` flag:

```bash
zitadel start --auto-migrate --masterkey "MasterKeyNeedsToBe32BytesLong!!"
```

### 4.2 Declarative Bootstrap Parameters
When a fresh database is detected (0 instances exist), the bootstrapper automatically creates the root instance, default organization, management project, and administrative user:

```yaml
Bootstrap:
  InstanceID: "zitadel"
  InstanceName: "ZITADEL Primary Instance"
  DefaultDomain: "auth.example.com"
  OrgID: "org-default"
  OrgName: "Default Organization"
  AdminUsername: "zitadel-admin"
  AdminEmail: "admin@example.com"
  AdminPassword: "InitialSuperPassword123!"
```
*Environment Variables*:
```bash
ZITADEL_BOOTSTRAP_INSTANCEID=zitadel
ZITADEL_BOOTSTRAP_DEFAULTDOMAIN=auth.example.com
ZITADEL_BOOTSTRAP_ADMINUSERNAME=zitadel-admin
ZITADEL_BOOTSTRAP_ADMINPASSWORD="InitialSuperPassword123!"
```

---

## 5. Headless Authentication Link Integration

Since ZITADEL operates as a pure API engine, user emails (e.g., password reset, email verification, domain claiming) construct URLs pointing to your own frontend portal:

```yaml
Login:
  DefaultPaths:
    BasePath: "https://auth.mycompany.com/"
    PasswordSetPath: "auth/reset-password?code={{.Code}}&userId={{.UserID}}&orgId={{.OrgID}}"
    EmailCodePath: "auth/verify-email?code={{.Code}}&userId={{.UserID}}&orgId={{.OrgID}}"
    OTPEmailPath: "auth/mfa-otp?code={{.Code}}&userId={{.UserID}}&sessionId={{.SessionID}}"
```
*Environment Variable*:
```bash
ZITADEL_LOGIN_DEFAULTPATHS_BASEPATH=https://auth.mycompany.com/
```
