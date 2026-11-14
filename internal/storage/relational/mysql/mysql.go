//go:build storage_all || storage_mysql

package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"github.com/zitadel/zitadel/internal/storage/relational"
)

func init() {
	p := &Provider{isHeatWave: false}
	relational.RegisterProvider("mysql", p)
	relational.RegisterProvider("heatwave", &Provider{isHeatWave: true})
}

type Provider struct {
	isHeatWave bool
}

func (p *Provider) Name() string {
	if p.isHeatWave {
		return "heatwave"
	}
	return "mysql"
}

func (p *Provider) Dialect() relational.Dialect {
	return &Dialect{isHeatWave: p.isHeatWave}
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
			port = 3306
		}
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
			cfg.User, cfg.Password, host, port, cfg.Database,
		)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql connect: %w", err)
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

type Dialect struct {
	isHeatWave bool
}

func (d *Dialect) Name() string {
	if d.isHeatWave {
		return "heatwave"
	}
	return "mysql"
}

func (d *Dialect) QuoteIdentifier(name string) string {
	return "`" + name + "`"
}

func (d *Dialect) Placeholder(n int) string {
	return "?"
}

func (d *Dialect) UpsertClause(table string, pks []string, cols []string) string {
	var updates []string
	for _, col := range cols {
		qCol := d.QuoteIdentifier(col)
		updates = append(updates, fmt.Sprintf("%s = VALUES(%s)", qCol, qCol))
	}
	return fmt.Sprintf("ON DUPLICATE KEY UPDATE %s", strings.Join(updates, ", "))
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
	if strings.Contains(errMsg, "Error 1062") || strings.Contains(errMsg, "Duplicate entry") {
		return relational.ErrDuplicateKey
	}
	return err
}
