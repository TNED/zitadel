//go:build storage_all || storage_oracle

package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/sijms/go-ora/v2"

	"github.com/zitadel/zitadel/internal/storage/relational"
)

func init() {
	p := &Provider{}
	relational.RegisterProvider("oracle", p)
	relational.RegisterProvider("oracle23ai", p)
}

type Provider struct{}

func (p *Provider) Name() string {
	return "oracle"
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
			port = 1521
		}
		dsn = fmt.Sprintf("oracle://%s:%s@%s:%d/%s",
			cfg.User, cfg.Password, host, port, cfg.Database,
		)
	}

	db, err := sql.Open("oracle", dsn)
	if err != nil {
		return nil, fmt.Errorf("oracle connect: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(30)
	}
	return db, db.PingContext(ctx)
}

type Dialect struct{}

func (d *Dialect) Name() string {
	return "oracle"
}

func (d *Dialect) QuoteIdentifier(name string) string {
	return `"` + strings.ToUpper(name) + `"`
}

func (d *Dialect) Placeholder(n int) string {
	return fmt.Sprintf(":%d", n)
}

func (d *Dialect) UpsertClause(table string, pks []string, cols []string) string {
	return "/* Oracle MERGE */"
}

func (d *Dialect) PaginationClause(offset, limit int) string {
	if offset < 0 {
		offset = 0
	}
	return fmt.Sprintf("OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, limit)
}

func (d *Dialect) TranslateError(err error) error {
	if err == nil {
		return nil
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "ORA-00001") || strings.Contains(errMsg, "unique constraint") {
		return relational.ErrDuplicateKey
	}
	return err
}
