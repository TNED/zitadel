package nosql

import (
	"context"
	"sync"

	"github.com/zitadel/zitadel/internal/storage"
)

// Config provides connection settings for NoSQL backends.
type Config struct {
	ProviderName string
	Region       string
	Endpoint     string
	Credentials  string
	ProjectID    string
	TableName    string
	Options      map[string]string
}

// Provider defines the factory interface that every NoSQL provider must implement.
type Provider interface {
	Name() string
	NewSessionRepository(ctx context.Context, cfg Config) (storage.SessionRepository, error)
	NewAuditRepository(ctx context.Context, cfg Config) (storage.AuditRepository, error)
}

var (
	providersMu sync.RWMutex
	providers   = make(map[string]Provider)
)

// RegisterProvider registers a NoSQL provider.
func RegisterProvider(name string, p Provider) {
	providersMu.Lock()
	defer providersMu.Unlock()
	if p == nil {
		panic("nosql: RegisterProvider p is nil")
	}
	providers[name] = p
}

// GetProvider retrieves a registered NoSQL provider.
func GetProvider(name string) (Provider, bool) {
	providersMu.RLock()
	defer providersMu.RUnlock()
	p, ok := providers[name]
	return p, ok
}

// ListProviders returns all registered NoSQL providers.
func ListProviders() []string {
	providersMu.RLock()
	defer providersMu.RUnlock()
	list := make([]string, 0, len(providers))
	for k := range providers {
		list = append(list, k)
	}
	return list
}
