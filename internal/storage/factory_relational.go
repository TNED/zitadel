//go:build !storage_dynamodb && !storage_firestore

package storage

import (
	"context"
	"fmt"

	relational_fallback "github.com/zitadel/zitadel/internal/storage/fallback/relational"
	"github.com/zitadel/zitadel/internal/storage/relational"
	"github.com/zitadel/zitadel/internal/storage/relational/core"
)

// NewEngine initializes a pure relational storage engine with isolated fallback sessions/audit.
func NewEngine(ctx context.Context, cfg Config) (Engine, error) {
	relProvider, ok := relational.GetProvider(cfg.RelationalDriver)
	if !ok {
		return nil, fmt.Errorf("relational provider %q not registered or not compiled in", cfg.RelationalDriver)
	}
	db, err := relProvider.Connect(ctx, cfg.RelationalConfig)
	if err != nil {
		return nil, fmt.Errorf("relational connect: %w", err)
	}

	sessRepo := relational_fallback.NewSessionRepository(db, relProvider.Dialect())
	auditRepo := relational_fallback.NewAuditRepository(db, relProvider.Dialect())

	return &DefaultEngine{
		db:           db,
		coreRepo:     core.NewRepository(db, relProvider.Dialect()),
		sessionsRepo: sessRepo,
		auditRepo:    auditRepo,
	}, nil
}
