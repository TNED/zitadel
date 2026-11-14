//go:build storage_all || storage_mssql

package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/microsoft/go-mssqldb"

	"github.com/zitadel/zitadel/internal/storage/relational"
)

func init() {
	p := &Provider{}
	relational.RegisterProvider("mssql", p)
	relational.RegisterProvider("sqlserver", p)
}

type Provider struct{}

func (p *Provider) Name() string {
	return "mssql"
}

func (p *Provider) Dialect() relational.Dialect {
	return &Dialect{}
}

func (p *Provider) Connect(ctx context.Context, cfg relational.Config) (*sql.DB, error) {
	dsn := cfg.DSN
	if dsn == "" {
		host := cfg.Host
		if host == "" {
			host = "localhost"
		}
		port := cfg.Port
		if port == 0 {
			port = 1433
		}
		dsn = fmt.Sprintf("server=%s;user id=%s;password=%s;port=%d;database=%s;encrypt=true;trustServerCertificate=true",
			host, cfg.User, cfg.Password, port, cfg.Database,
		)
	}

	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return nil, fmt.Errorf("mssql connect: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(50)
	}
	return db, db.PingContext(ctx)
}

type Dialect struct{}

func (d *Dialect) Name() string {
	return "mssql"
}

func (d *Dialect) QuoteIdentifier(name string) string {
	return "[" + name + "]"
}

func (d *Dialect) Placeholder(n int) string {
	return fmt.Sprintf("@p%d", n)
}

func (d *Dialect) UpsertClause(table string, pks []string, cols []string) string {
	// Standard SQL Server MERGE is typically structured as a full statement.
	// For inline dialect clause we provide a comment hint.
	return "/* MSSQL MERGE */"
}

func (d *Dialect) PaginationClause(offset, limit int) string {
	if offset < 0 {
		offset = 0
	}
	return fmt.Sprintf("ORDER BY (SELECT NULL) OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, limit)
}

func (d *Dialect) TranslateError(err error) error {
	if err == nil {
		return nil
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "2627") || strings.Contains(errMsg, "2601") || strings.Contains(errMsg, "duplicate key") {
		return relational.ErrDuplicateKey
	}
	return err
}
