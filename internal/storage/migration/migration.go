package migration

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	"github.com/zitadel/zitadel/internal/storage"
)

//go:embed schema_sqlite.sql
var schemaSQLite string

//go:embed schema_postgres.sql
var schemaPostgres string

//go:embed schema_dsql.sql
var schemaDSQL string

//go:embed schema_mysql.sql
var schemaMySQL string

//go:embed schema_mssql.sql
var schemaMSSQL string

//go:embed schema_oracle.sql
var schemaOracle string

//go:embed schema_relational_session_audit.sql
var schemaRelationalSessionAudit string

// Runner handles dialect-specific idempotent DDL migrations.
type Runner struct {
	db               *sql.DB
	dialect          storage.Dialect
	isPureRelational bool
}

// NewRunner creates a new migration runner for a database connection.
func NewRunner(db *sql.DB, dialect storage.Dialect) *Runner {
	return &Runner{
		db:               db,
		dialect:          dialect,
		isPureRelational: DefaultIsPureRelational,
	}
}

// SetPureRelational explicitly overrides the pure-relational mode flag.
func (r *Runner) SetPureRelational(val bool) *Runner {
	r.isPureRelational = val
	return r
}

// Run executes the core schema DDL and optional session/audit fallback tables.
func (r *Runner) Run(ctx context.Context) error {
	// 1. Get dialect DDL script
	ddl, err := r.getSchemaDDL()
	if err != nil {
		return err
	}

	// 2. Execute dialect core DDL
	if err := r.executeScript(ctx, ddl); err != nil {
		return fmt.Errorf("migration failed on %s dialect: %w", r.dialect, err)
	}

	// 3. Conditionally execute relational session/audit DDL if pure relational mode is active
	// Note: For Oracle dialects (including 23ai and 26ai), session and audit tables are natively defined in schemaOracle
	// using Oracle-compliant types (CLOB, TIMESTAMP WITH TIME ZONE) rather than generic TEXT.
	if r.isPureRelational && !r.isOracleDialect() {
		if err := r.executeScript(ctx, schemaRelationalSessionAudit); err != nil {
			return fmt.Errorf("migration failed for relational session/audit tables: %w", err)
		}
	}

	// 4. Record schema version
	if err := r.recordVersion(ctx, 1, "initial core schema"); err != nil {
		// Log or ignore if version record already exists
		return nil
	}

	return nil
}

func (r *Runner) getSchemaDDL() (string, error) {
	switch r.dialect {
	case storage.DialectSQLite:
		return schemaSQLite, nil
	case storage.DialectPostgres:
		return schemaPostgres, nil
	case storage.DialectDSQL:
		return schemaDSQL, nil
	case storage.DialectMySQL:
		return schemaMySQL, nil
	case storage.DialectMSSQL:
		return schemaMSSQL, nil
	case storage.DialectOracle, storage.DialectOracle23ai, storage.DialectOracle26ai:
		return schemaOracle, nil
	default:
		return "", fmt.Errorf("unsupported database dialect for migration: %s", r.dialect)
	}
}

func (r *Runner) executeScript(ctx context.Context, script string) error {
	statements := splitSQLStatements(script)
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("executing DDL statement failed [%s]: %w", stmt, err)
		}
	}
	return nil
}

func (r *Runner) recordVersion(ctx context.Context, version int, description string) error {
	query := `INSERT INTO zitadel_schema_version (version, description) VALUES (?, ?) ON CONFLICT DO NOTHING;`
	if r.dialect == storage.DialectPostgres || r.dialect == storage.DialectDSQL || r.dialect == storage.DialectSQLite {
		query = `INSERT INTO zitadel_schema_version (version, description) VALUES ($1, $2) ON CONFLICT (version) DO NOTHING;`
		_, err := r.db.ExecContext(ctx, query, version, description)
		return err
	} else if r.dialect == storage.DialectMySQL {
		query = `INSERT IGNORE INTO zitadel_schema_version (version, description) VALUES (?, ?);`
		_, err := r.db.ExecContext(ctx, query, version, description)
		return err
	} else if r.dialect == storage.DialectMSSQL {
		query = `IF NOT EXISTS (SELECT 1 FROM zitadel_schema_version WHERE version = @p1) INSERT INTO zitadel_schema_version (version, description) VALUES (@p1, @p2);`
		_, err := r.db.ExecContext(ctx, query, version, description)
		return err
	} else if r.isOracleDialect() {
		query = `MERGE INTO zitadel_schema_version tgt USING (SELECT :1 AS version, :2 AS description FROM dual) src ON (tgt.version = src.version) WHEN NOT MATCHED THEN INSERT (version, description) VALUES (src.version, src.description);`
		_, err := r.db.ExecContext(ctx, query, version, description)
		return err
	}
	_, err := r.db.ExecContext(ctx, query, version, description)
	return err
}

func (r *Runner) isOracleDialect() bool {
	return r.dialect == storage.DialectOracle || r.dialect == storage.DialectOracle23ai || r.dialect == storage.DialectOracle26ai
}

func splitSQLStatements(script string) []string {
	var statements []string
	lines := strings.Split(script, "\n")
	var current strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			stmt := strings.TrimSuffix(strings.TrimSpace(current.String()), ";")
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
		}
	}
	if rem := strings.TrimSpace(current.String()); rem != "" {
		statements = append(statements, rem)
	}
	return statements
}
