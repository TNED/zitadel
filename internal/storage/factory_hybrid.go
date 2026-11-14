//go:build storage_dynamodb || storage_firestore

package storage

import (
	"context"
	"fmt"

	"github.com/zitadel/zitadel/internal/storage/nosql"
	"github.com/zitadel/zitadel/internal/storage/relational"
	"github.com/zitadel/zitadel/internal/storage/relational/core"
)

// NewEngine initializes a hybrid storage engine (Relational Core + NoSQL Sessions/Audit).
func NewEngine(ctx context.Context, cfg Config) (Engine, error) {
	relProvider, ok := relational.GetProvider(cfg.RelationalDriver)
	if !ok {
		return nil, fmt.Errorf("relational provider %q not registered or not compiled in", cfg.RelationalDriver)
	}
	db, err := relProvider.Connect(ctx, cfg.RelationalConfig)
	if err != nil {
		return nil, fmt.Errorf("relational connect: %w", err)
	}

	nosqlProvider, ok := nosql.GetProvider(cfg.NoSQLDriver)
	if !ok {
		return nil, fmt.Errorf("nosql provider %q not registered or not compiled in", cfg.NoSQLDriver)
	}
	var nosqlCfg nosql.Config
	if c, ok := cfg.NoSQLConfig.(nosql.Config); ok {
		nosqlCfg = c
	}
	sessRepo, err := nosqlProvider.NewSessionRepository(ctx, nosqlCfg)
	if err != nil {
		return nil, fmt.Errorf("nosql session repo: %w", err)
	}
	auditRepo, err := nosqlProvider.NewAuditRepository(ctx, nosqlCfg)
	if err != nil {
		return nil, fmt.Errorf("nosql audit repo: %w", err)
	}

	return &DefaultEngine{
		db:           db,
		coreRepo:     core.NewRepository(db, relProvider.Dialect()),
		sessionsRepo: sessRepo,
		auditRepo:    auditRepo,
	}, nil
}
