//go:build storage_all || storage_postgres

package postgres

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
	relational.RegisterProvider("postgres", p)
	relational.RegisterProvider("postgresql", p)
}

type Provider struct{}

func (p *Provider) Name() string {
	return "postgres"
}

func (p *Provider) Dialect() relational.Dialect {
	return &Dialect{}
}

func (p *Provider) Connect(ctx context.Context, cfg relational.Config) (*sql.DB, error) {
	dsn := cfg.DSN
	if dsn == "" {
		host := cfg.Host
		if host == "" {
			host = "127.0.0.1"
		}
		port := cfg.Port
		if port == 0 {
			port = 5432
		}
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
			cfg.User, cfg.Password, host, port, cfg.Database,
		)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres connect: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(50)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(10)
	}
	return db, db.PingContext(ctx)
}

type Dialect struct{}

func (d *Dialect) Name() string {
	return "postgres"
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
	if strings.Contains(errMsg, "23505") || strings.Contains(errMsg, "duplicate key value") {
		return relational.ErrDuplicateKey
	}
	return err
}
