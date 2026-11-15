# ZITADEL Monorepo Guide for AI Agents

## Mission & Context
ZITADEL is an open-source Identity Management System (IAM) written in Go. It provides secure headless authentication, APIs, multi-tenancy, and audit trails.

## Read Order
1. Read this file first.
2. Read the nearest scoped `AGENTS.md` for the area you edit.
3. If multiple scopes apply, use the most specific path.

## Repository Structure Map
- **`cmd/`**: Entrypoint and CLI commands for the Zitadel service.
- **`internal/`**: Backend domain and service logic. See `internal/AGENTS.md`.
  - **`storage/`**: Pluggable storage SPI supporting 7 relational databases (SQLite, MySQL, PostgreSQL, MSSQL, Oracle, DSQL, HeatWave) and multi-cloud NoSQL (DynamoDB, Firestore).
- **`proto/`**: API definitions. See `proto/AGENTS.md`.

## Technology Stack & Conventions
- **Orchestration**: Makefile is used for build and task orchestration.
- **Backend**:
  - **Go Version Source of Truth**: Inspect `go.mod` before Go work (`go` and optional `toolchain` directives).
  - **Communication**: For V2 APIs, connectRPC is the primary transport. gRPC and HTTP/JSON endpoints are also supported.
  - **Architecture**: Pure Headless API service with pluggable relational and NoSQL storage SPI. Audit logs are decoupled and asynchronous.
- **Storage & Multi-Database**:
  - **Relational**: SQLite, PostgreSQL, SQL Server, MySQL, MySQL HeatWave, Aurora DSQL, Oracle 23ai.
  - **NoSQL (Optional)**: Amazon DynamoDB, Google Cloud Firestore.
  - **Build Tags**: Modules are selected via Go build tags (`storage_sqlite`, `storage_mysql`, `storage_postgres`, `storage_mssql`, `storage_dsql`, `storage_oracle`, `storage_dynamodb`, `storage_firestore`).

## ZITADEL Domain & Multi-Tenancy Logic

### 1. Hierarchy & Ownership

ZITADEL follows a strict hierarchical containment model. When generating code, logic, or translations, adhere to this structure:

- **System (Installation):** The entire ZITADEL deployment. Global settings are applied through runtime configuration files or environment variables. See `cmd/defaults.yaml`.
- **Instance (The "Identity System"):**
  - **Definition:** A logical partition/virtual tenant. It is a "System inside a System."
  - **Isolation:** Data and settings are strictly isolated between instances.
  - **Translation Rule:** NEVER translate as "Example" or "Case." Use technical terms like "Tenant," "Environment," or the local equivalent of "Logical System Entity."
- **Organization:** A group within an Instance. It owns Users, Projects, and Roles.
- **Project:** A collection of Applications and Auth Policies within an Org.

### 2. Permission Scoping (The "Administrative" Context)

- **System User:** Manages the entire Installation and creates Instances over the system API.
- **Instance Admin:** Manages Instance-wide policies (Password complexity, Identity Providers, Organizations).
- **Organization Admin:** Manages users and access within a specific Organization.

### 3. Language & Tone Guidelines

- **Avoid Ambiguity:** When referring to an 'Instance', the context is always infrastructure/tenancy.
- **Technical Precision:** In UI strings, prefer clarity over brevity if "Instance" is likely to be misinterpreted in the target language.

### 4. Technical Glossary & Localization Mapping

| Language             | Technical Term (SaaS/Cloud) | Why this term?                                            | "Avoid this (The ""Example"" Trap)" |
| -------------------- | --------------------------- | --------------------------------------------------------- | ----------------------------------- |
| Chinese (Simplified) | 实例 (Shílì)                | Standard for a cloud resource/entity.                     | 例子 (Lìzi)                         |
| Japanese             | インスタンス (Insutansu)    | Katakana transliteration; industry standard.              | 例 (Rei)                            |
| Korean               | 인스턴스 (Inseuteonseu)     | Hangul transliteration; industry standard.                | 예 (Ye)                             |
| German               | Instanz                     | Matches English but implies a technical occurrence.       | Beispiel                            |
| French               | Instance                    | "Standard but often requires ""de ZITADEL"" for clarity." | Exemple                             |
| Spanish              | Instancia                   | Technical entity in software architecture.                | Ejemplo                             |
| Portuguese           | Instância                   | Standard technical terminology.                           | Exemplo                             |
| Russian              | Инстанс (Instans)           | Modern SaaS jargon (transliterated).                      | Пример (Primer)                     |

#### Translation Guardrails
If a translation is requested for a language not listed above, follow these priority rules for the word 'Instance':

1. **Priority 1 (Transliteration):** Use the phonetic transliteration into the local script (common in Japanese/Korean/Russian).
2. **Priority 2 (System Entity):** Use a term that implies a "running process" or "logical environment."
3. **Priority 3 (Tenant):** If 'Instance' is ambiguous, use the local word for 'Tenant' (e.g., 租户 in Chinese).
4. **Strict Ban:** NEVER use words that mean "an illustration", "a case", "a sample", or "an example."

### 5. Deployment Targets

ZITADEL supports containerized deployments and orchestration:
- **Container / Dockerfile**: Pure Scratch image built via repository `Dockerfile` and `make docker`.
- **Kubernetes (Helm)**: External ([zitadel-charts](https://github.com/zitadel/zitadel-charts)).
- The same `ZITADEL_*` environment variable model applies across all deployment methods.

## Command Rules
Run commands from the repository root using Makefile targets:

- `make build`: Compile the zitadel binary into `.artifacts/bin/$(go env GOOS)/$(go env GOARCH)/zitadel`.
- `make build-linux`: Cross-compile for Linux AMD64.
- `make test`: Run unit tests (`make test-unit`).
- `make test-integration`: Run integration tests.
- `make lint`: Run `golangci-lint` with repository rules.
- `make lint-fix`: Automatically fix lint issues where supported.
- `make generate`: Regenerate protobuf, ConnectRPC, and statik assets.
- `make docker`: Build local Docker image.
- `make clean`: Clean build artifacts and coverage reports.

## PR Title Convention

PR titles are validated by the Semantic PR app. Format:

`<type>(<scope>): <short summary>`

**Types**: must come from the list in [`.github/semantic.yml`](.github/semantic.yml) under `types:` — e.g. `feat`, `fix`, `docs`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.

**Scopes**: optional, but if used must come from the list in [`.github/semantic.yml`](.github/semantic.yml) under `scopes:`. When in doubt, omit the scope — do not invent values not on that list.

## Skills
Tool-neutral runbooks live in `skills/` (see `skills/README.md`). Read the matching one before starting such a task.
- **`skills/npm-dependency-audit.md`**: Fix `pnpm audit` findings at their origin, prune `pnpm-workspace.yaml` overrides and prepare a version-bump PR.
- **`skills/test-actor-in-action-v2.md`**: Verify the impersonation actor reaches Actions v2 targets on a local instance.

## Documentation
- **Human Guide**: See `CONTRIBUTING.md` for setup and contribution details.
- **API Design**: See `API_DESIGN.md` for API specific guidelines.
