//go:build storage_all || storage_oracle

package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	_ "github.com/sijms/go-ora/v2"

	"github.com/zitadel/zitadel/internal/storage/relational"
)

func init() {
	relational.RegisterProvider("oracle", &Provider{name: "oracle"})
	relational.RegisterProvider("oracle23ai", &Provider{name: "oracle23ai"})
	relational.RegisterProvider("oracle26ai", &Provider{name: "oracle26ai"})
}

type Provider struct {
	name string
}

func (p *Provider) Name() string {
	if p.name == "" {
		return "oracle"
	}
	return p.name
}

func (p *Provider) Dialect() relational.Dialect {
	return &Dialect{name: p.Name()}
}

// BuildDSN constructs an Oracle DSN from configuration parameters, supporting OCI Autonomous Database (ATP/AJD)
// options like SSL and WALLET.
func BuildDSN(cfg relational.Config) string {
	if cfg.DSN != "" {
		return cfg.DSN
	}
	host := cfg.Host
	if host == "" {
		host = "localhost"
	}
	port := cfg.Port
	if port == 0 {
		port = 1521
	}
	dbName := cfg.Database
	if dbName == "" {
		dbName = "FREEPDB1"
	}

	dsn := fmt.Sprintf("oracle://%s:%s@%s:%d/%s",
		url.QueryEscape(cfg.User), url.QueryEscape(cfg.Password), host, port, dbName,
	)

	if len(cfg.Options) > 0 {
		params := url.Values{}
		for k, v := range cfg.Options {
			params.Set(k, v)
		}
		dsn = fmt.Sprintf("%s?%s", dsn, params.Encode())
	}

	return dsn
}

func (p *Provider) Connect(ctx context.Context, cfg relational.Config) (*sql.DB, error) {
	dsn := BuildDSN(cfg)

	db, err := sql.Open("oracle", dsn)
	if err != nil {
		return nil, fmt.Errorf("oracle connect: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(30)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	return db, db.PingContext(ctx)
}

type Dialect struct {
	name string
}

func (d *Dialect) Name() string {
	if d.name == "" {
		return "oracle"
	}
	return d.name
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
