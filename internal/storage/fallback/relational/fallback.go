//go:build !storage_dynamodb && !storage_firestore

package relational_fallback

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/zitadel/zitadel/internal/storage"
	"github.com/zitadel/zitadel/internal/storage/relational"
)

// SessionRepo implements storage.SessionRepository using a relational database table.
// Only compiled and used when No NoSQL driver is active.
type SessionRepo struct {
	db      *sql.DB
	dialect relational.Dialect
}

func NewSessionRepository(db *sql.DB, dialect relational.Dialect) *SessionRepo {
	r := &SessionRepo{db: db, dialect: dialect}
	r.startJanitor(context.Background())
	return r
}

func (r *SessionRepo) CreateSession(ctx context.Context, s *storage.Session) error {
	dataJSON, _ := json.Marshal(s.Data)
	query := fmt.Sprintf(
		"INSERT INTO zitadel_sessions (%s, %s, %s, %s, %s, %s, %s, %s) VALUES (%s, %s, %s, %s, %s, %s, %s, %s)",
		r.dialect.QuoteIdentifier("id"),
		r.dialect.QuoteIdentifier("instance_id"),
		r.dialect.QuoteIdentifier("user_id"),
		r.dialect.QuoteIdentifier("user_agent"),
		r.dialect.QuoteIdentifier("ip_address"),
		r.dialect.QuoteIdentifier("data"),
		r.dialect.QuoteIdentifier("created_at"),
		r.dialect.QuoteIdentifier("expires_at"),
		r.dialect.Placeholder(1),
		r.dialect.Placeholder(2),
		r.dialect.Placeholder(3),
		r.dialect.Placeholder(4),
		r.dialect.Placeholder(5),
		r.dialect.Placeholder(6),
		r.dialect.Placeholder(7),
		r.dialect.Placeholder(8),
	)
	_, err := r.db.ExecContext(ctx, query,
		s.ID, s.InstanceID, s.UserID, s.UserAgent, s.IPAddress, string(dataJSON), s.CreatedAt, s.ExpiresAt,
	)
	return r.dialect.TranslateError(err)
}

func (r *SessionRepo) GetSession(ctx context.Context, instanceID, sessionID string) (*storage.Session, error) {
	// Query-time exclusion: strictly filter WHERE expires_at > NOW()
	query := fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s, %s, %s FROM zitadel_sessions WHERE %s = %s AND %s = %s AND %s > %s",
		r.dialect.QuoteIdentifier("id"),
		r.dialect.QuoteIdentifier("instance_id"),
		r.dialect.QuoteIdentifier("user_id"),
		r.dialect.QuoteIdentifier("user_agent"),
		r.dialect.QuoteIdentifier("ip_address"),
		r.dialect.QuoteIdentifier("data"),
		r.dialect.QuoteIdentifier("created_at"),
		r.dialect.QuoteIdentifier("expires_at"),
		r.dialect.QuoteIdentifier("instance_id"), r.dialect.Placeholder(1),
		r.dialect.QuoteIdentifier("id"), r.dialect.Placeholder(2),
		r.dialect.QuoteIdentifier("expires_at"), r.dialect.Placeholder(3),
	)

	row := r.db.QueryRowContext(ctx, query, instanceID, sessionID, time.Now())
	s := &storage.Session{}
	var dataStr string
	err := row.Scan(&s.ID, &s.InstanceID, &s.UserID, &s.UserAgent, &s.IPAddress, &dataStr, &s.CreatedAt, &s.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, r.dialect.TranslateError(err)
	}
	if dataStr != "" {
		_ = json.Unmarshal([]byte(dataStr), &s.Data)
	}
	return s, nil
}

func (r *SessionRepo) UpdateSession(ctx context.Context, s *storage.Session) error {
	dataJSON, _ := json.Marshal(s.Data)
	query := fmt.Sprintf(
		"UPDATE zitadel_sessions SET %s = %s, %s = %s, %s = %s, %s = %s WHERE %s = %s AND %s = %s",
		r.dialect.QuoteIdentifier("user_agent"), r.dialect.Placeholder(1),
		r.dialect.QuoteIdentifier("ip_address"), r.dialect.Placeholder(2),
		r.dialect.QuoteIdentifier("data"), r.dialect.Placeholder(3),
		r.dialect.QuoteIdentifier("expires_at"), r.dialect.Placeholder(4),
		r.dialect.QuoteIdentifier("instance_id"), r.dialect.Placeholder(5),
		r.dialect.QuoteIdentifier("id"), r.dialect.Placeholder(6),
	)
	_, err := r.db.ExecContext(ctx, query, s.UserAgent, s.IPAddress, string(dataJSON), s.ExpiresAt, s.InstanceID, s.ID)
	return r.dialect.TranslateError(err)
}

func (r *SessionRepo) DeleteSession(ctx context.Context, instanceID, sessionID string) error {
	query := fmt.Sprintf(
		"DELETE FROM zitadel_sessions WHERE %s = %s AND %s = %s",
		r.dialect.QuoteIdentifier("instance_id"), r.dialect.Placeholder(1),
		r.dialect.QuoteIdentifier("id"), r.dialect.Placeholder(2),
	)
	_, err := r.db.ExecContext(ctx, query, instanceID, sessionID)
	return r.dialect.TranslateError(err)
}

// startJanitor launches a background goroutine to asynchronously clean up expired rows
func (r *SessionRepo) startJanitor(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				query := fmt.Sprintf(
					"DELETE FROM zitadel_sessions WHERE %s < %s",
					r.dialect.QuoteIdentifier("expires_at"), r.dialect.Placeholder(1),
				)
				_, _ = r.db.ExecContext(context.Background(), query, time.Now())
			}
		}
	}()
}

// AuditRepo implements storage.AuditRepository using relational database with buffered batch inserter.
type AuditRepo struct {
	db      *sql.DB
	dialect relational.Dialect
	buffer  chan *storage.AuditEntry
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewAuditRepository(db *sql.DB, dialect relational.Dialect) *AuditRepo {
	ctx, cancel := context.WithCancel(context.Background())
	a := &AuditRepo{
		db:      db,
		dialect: dialect,
		buffer:  make(chan *storage.AuditEntry, 1000),
		ctx:     ctx,
		cancel:  cancel,
	}
	a.startBatchWriter()
	return a
}

func (a *AuditRepo) Log(ctx context.Context, entry *storage.AuditEntry) error {
	select {
	case a.buffer <- entry:
		return nil
	default:
		// If buffer full, write synchronously
		return a.writeSingle(ctx, entry)
	}
}

func (a *AuditRepo) writeSingle(ctx context.Context, entry *storage.AuditEntry) error {
	detailsJSON, _ := json.Marshal(entry.Details)
	query := fmt.Sprintf(
		"INSERT INTO zitadel_audit_logs (%s, %s, %s, %s, %s, %s, %s, %s, %s) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)",
		a.dialect.QuoteIdentifier("id"),
		a.dialect.QuoteIdentifier("instance_id"),
		a.dialect.QuoteIdentifier("actor_id"),
		a.dialect.QuoteIdentifier("action"),
		a.dialect.QuoteIdentifier("resource_type"),
		a.dialect.QuoteIdentifier("resource_id"),
		a.dialect.QuoteIdentifier("status"),
		a.dialect.QuoteIdentifier("details"),
		a.dialect.QuoteIdentifier("created_at"),
		a.dialect.Placeholder(1),
		a.dialect.Placeholder(2),
		a.dialect.Placeholder(3),
		a.dialect.Placeholder(4),
		a.dialect.Placeholder(5),
		a.dialect.Placeholder(6),
		a.dialect.Placeholder(7),
		a.dialect.Placeholder(8),
		a.dialect.Placeholder(9),
	)
	_, err := a.db.ExecContext(ctx, query,
		entry.ID, entry.InstanceID, entry.ActorID, entry.Action, entry.ResourceType, entry.ResourceID, entry.Status, string(detailsJSON), entry.Timestamp,
	)
	return a.dialect.TranslateError(err)
}

func (a *AuditRepo) startBatchWriter() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		var batch []*storage.AuditEntry
		flush := func() {
			if len(batch) == 0 {
				return
			}
			for _, entry := range batch {
				_ = a.writeSingle(context.Background(), entry)
			}
			batch = batch[:0]
		}

		for {
			select {
			case <-a.ctx.Done():
				flush()
				return
			case entry := <-a.buffer:
				batch = append(batch, entry)
				if len(batch) >= 100 {
					flush()
				}
			case <-ticker.C:
				flush()
			}
		}
	}()
}

func (a *AuditRepo) Query(ctx context.Context, filter storage.AuditFilter) ([]*storage.AuditEntry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	query := fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s, %s, %s, %s FROM zitadel_audit_logs WHERE %s = %s ORDER BY %s DESC %s",
		a.dialect.QuoteIdentifier("id"),
		a.dialect.QuoteIdentifier("instance_id"),
		a.dialect.QuoteIdentifier("actor_id"),
		a.dialect.QuoteIdentifier("action"),
		a.dialect.QuoteIdentifier("resource_type"),
		a.dialect.QuoteIdentifier("resource_id"),
		a.dialect.QuoteIdentifier("status"),
		a.dialect.QuoteIdentifier("details"),
		a.dialect.QuoteIdentifier("created_at"),
		a.dialect.QuoteIdentifier("instance_id"), a.dialect.Placeholder(1),
		a.dialect.QuoteIdentifier("created_at"),
		a.dialect.PaginationClause(0, limit),
	)

	rows, err := a.db.QueryContext(ctx, query, filter.InstanceID)
	if err != nil {
		return nil, a.dialect.TranslateError(err)
	}
	defer rows.Close()

	var entries []*storage.AuditEntry
	for rows.Next() {
		e := &storage.AuditEntry{}
		var detailsStr string
		if err := rows.Scan(&e.ID, &e.InstanceID, &e.ActorID, &e.Action, &e.ResourceType, &e.ResourceID, &e.Status, &detailsStr, &e.Timestamp); err != nil {
			return nil, err
		}
		if detailsStr != "" {
			_ = json.Unmarshal([]byte(detailsStr), &e.Details)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
