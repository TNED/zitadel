//go:build storage_all || storage_sqlite

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/zitadel/zitadel/internal/storage/relational"
)

func init() {
	p := &Provider{}
	relational.RegisterProvider("sqlite", p)
	relational.RegisterProvider("sqlite3", p)
}

type Provider struct{}

func (p *Provider) Name() string {
	return "sqlite"
}

func (p *Provider) Dialect() relational.Dialect {
	return &Dialect{}
}

func (p *Provider) Connect(ctx context.Context, cfg relational.Config) (*sql.DB, error) {
	dsn := cfg.DSN
	if dsn == "" {
		dsn = cfg.Database
	}
	if dsn == "" {
		dsn = "zitadel.sqlite"
	}
	// Append WAL pragma if not present
	if !strings.Contains(dsn, "_pragma=journal_mode") {
		if strings.Contains(dsn, "?") {
			dsn += "&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
		} else {
			dsn += "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
		}
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite connect: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	} else {
		// WAL mode supports concurrent readers, but 1 writer at a time
		db.SetMaxOpenConns(10)
	}
	return db, db.PingContext(ctx)
}

type Dialect struct{}

func (d *Dialect) Name() string {
	return "sqlite"
}

func (d *Dialect) QuoteIdentifier(name string) string {
	return `"` + name + `"`
}

func (d *Dialect) Placeholder(n int) string {
	return "?"
}

func (d *Dialect) UpsertClause(table string, pks []string, cols []string) string {
	var pkList, updates []string
	for _, pk := range pks {
		pkList = append(pkList, d.QuoteIdentifier(pk))
	}
	for _, col := range cols {
		updates = append(updates, fmt.Sprintf("%s = excluded.%s", d.QuoteIdentifier(col), d.QuoteIdentifier(col)))
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
	if strings.Contains(errMsg, "UNIQUE constraint failed") || strings.Contains(errMsg, "constraint failed") {
		return relational.ErrDuplicateKey
	}
	return err
}
