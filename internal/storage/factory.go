package storage

import (
	"database/sql"
	"errors"
	"io"

	"github.com/zitadel/zitadel/internal/storage/relational"
	"github.com/zitadel/zitadel/internal/storage/relational/core"
)

// Config configures storage engine initialization.
type Config struct {
	RelationalDriver string
	RelationalConfig relational.Config
	NoSQLDriver      string
	NoSQLConfig      any
}

// DefaultEngine coordinates domain repositories.
type DefaultEngine struct {
	db           *sql.DB
	coreRepo     *core.Repository
	sessionsRepo SessionRepository
	auditRepo    AuditRepository
}

func (e *DefaultEngine) Users() UserRepository {
	return e.coreRepo
}

func (e *DefaultEngine) Orgs() OrgRepository {
	return e.coreRepo
}

func (e *DefaultEngine) Projects() ProjectRepository {
	return e.coreRepo
}

func (e *DefaultEngine) Sessions() SessionRepository {
	return e.sessionsRepo
}

func (e *DefaultEngine) Audit() AuditRepository {
	return e.auditRepo
}

func (e *DefaultEngine) Close() error {
	var errs []error
	if closer, ok := e.sessionsRepo.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if closer, ok := e.auditRepo.(io.Closer); ok {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if e.db != nil {
		if err := e.db.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
