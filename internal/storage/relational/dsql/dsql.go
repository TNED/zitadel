//go:build storage_all || storage_dsql

package dsql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/zitadel/zitadel/internal/storage/relational"
)

func init() {
	p := &Provider{}
	relational.RegisterProvider("dsql", p)
	relational.RegisterProvider("aurora_dsql", p)
}

type Provider struct{}

func (p *Provider) Name() string {
	return "dsql"
}

func (p *Provider) Dialect() relational.Dialect {
	return &Dialect{}
}

func (p *Provider) Connect(ctx context.Context, cfg relational.Config) (*sql.DB, error) {
	dsn := cfg.DSN
	if dsn == "" {
		host := cfg.Host
		port := cfg.Port
		if port == 0 {
			port = 5432
		}
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=require",
			cfg.User, cfg.Password, host, port, cfg.Database,
		)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("aurora dsql connect: %w", err)
	}
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(20)
	return db, db.PingContext(ctx)
}

type Dialect struct{}

func (d *Dialect) Name() string {
	return "dsql"
}

func (d *Dialect) QuoteIdentifier(name string) string {
	return `"` + name + `"`
}

func (d *Dialect) Placeholder(n int) string {
	return fmt.Sprintf("$%d", n)
}

func (d *Dialect) UpsertClause(table string, pks []string, cols []string) string {
	var pkList, updates []string
	for _, pk := range pks {
		pkList = append(pkList, d.QuoteIdentifier(pk))
	}
	for _, col := range cols {
		updates = append(updates, fmt.Sprintf("%s = EXCLUDED.%s", d.QuoteIdentifier(col), d.QuoteIdentifier(col)))
	}
	return fmt.Sprintf("ON CONFLICT (%s) DO UPDATE SET %s", strings.Join(pkList, ", "), strings.Join(updates, ", "))
}

func (d *Dialect) PaginationClause(offset, limit int) string {
	if offset > 0 {
		return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
	}
	return fmt.Sprintf("LIMIT %d", limit)
}

func (d *Dialect) TranslateError(err error) error {
	if err == nil {
		return nil
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "23505") || strings.Contains(errMsg, "duplicate key") {
		return relational.ErrDuplicateKey
	}
	// Aurora DSQL specific OCC transaction conflict
	if strings.Contains(errMsg, "40001") || strings.Contains(errMsg, "serialization_failure") {
		return fmt.Errorf("aurora dsql occ conflict (retry suggested): %w", err)
	}
	return err
}
