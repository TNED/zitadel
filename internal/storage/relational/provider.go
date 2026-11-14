package relational

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrDuplicateKey = errors.New("duplicate key / unique constraint violation")
	ErrNotFound     = errors.New("record not found")
)

// Dialect defines SQL syntax variations across different relational database engines.
type Dialect interface {
	Name() string
	QuoteIdentifier(name string) string
	Placeholder(n int) string // 1-based index: 1 -> "?", "$1", ":1", "@p1"
	UpsertClause(table string, pks []string, cols []string) string
	PaginationClause(offset, limit int) string
	TranslateError(err error) error
}

// Config provides standard connection parameters for relational databases.
type Config struct {
	DriverName     string
	DSN            string
	Host           string
	Port           int
	Database       string
	User           string
	Password       string
	MaxOpenConns   int
	MaxIdleConns   int
	ConnectTimeout int
	Options        map[string]string
}

// Provider defines the SPI factory for registering relational database drivers.
type Provider interface {
	Name() string
	Dialect() Dialect
	Connect(ctx context.Context, cfg Config) (*sql.DB, error)
}

var (
	providersMu sync.RWMutex
	providers   = make(map[string]Provider)
)

// RegisterProvider registers a relational database provider SPI.
func RegisterProvider(name string, p Provider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	if p == nil {
		panic("relational: RegisterProvider p is nil")
	}
	providers[name] = p
}

// GetProvider retrieves a registered relational provider.
func GetProvider(name string) (Provider, bool) {
	providersMu.RLock()
	defer providersMu.RUnlock()
	p, ok := providers[name]
	return p, ok
}

// ListProviders returns all registered relational database names.
func ListProviders() []string {
	providersMu.RLock()
	defer providersMu.RUnlock()
	list := make([]string, 0, len(providers))
	for k := range providers {
		list = append(list, k)
	}
	return list
}
