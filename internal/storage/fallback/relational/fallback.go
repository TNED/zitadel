//go:build !storage_dynamodb && !storage_firestore

package relational_fallback

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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

	qCreateSession string
	qGetSession    string
	qUpdateSession string
	qDeleteSession string
	qJanitor       string

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewSessionRepository(db *sql.DB, dialect relational.Dialect) *SessionRepo {
	ctx, cancel := context.WithCancel(context.Background())
	r := &SessionRepo{
		db:      db,
		dialect: dialect,
		ctx:     ctx,
		cancel:  cancel,
	}
	r.initQueries(dialect)
	r.startJanitor()
	return r
}

func (r *SessionRepo) initQueries(d relational.Dialect) {
	r.qCreateSession = fmt.Sprintf(
		"INSERT INTO zitadel_sessions (%s, %s, %s, %s, %s, %s, %s, %s) VALUES (%s, %s, %s, %s, %s, %s, %s, %s)",
		d.QuoteIdentifier("id"),
		d.QuoteIdentifier("instance_id"),
		d.QuoteIdentifier("user_id"),
		d.QuoteIdentifier("user_agent"),
		d.QuoteIdentifier("ip_address"),
		d.QuoteIdentifier("data"),
		d.QuoteIdentifier("created_at"),
		d.QuoteIdentifier("expires_at"),
		d.Placeholder(1),
		d.Placeholder(2),
		d.Placeholder(3),
		d.Placeholder(4),
		d.Placeholder(5),
		d.Placeholder(6),
		d.Placeholder(7),
		d.Placeholder(8),
	)

	r.qGetSession = fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s, %s, %s FROM zitadel_sessions WHERE %s = %s AND %s = %s AND %s > %s",
		d.QuoteIdentifier("id"),
		d.QuoteIdentifier("instance_id"),
		d.QuoteIdentifier("user_id"),
		d.QuoteIdentifier("user_agent"),
		d.QuoteIdentifier("ip_address"),
		d.QuoteIdentifier("data"),
		d.QuoteIdentifier("created_at"),
		d.QuoteIdentifier("expires_at"),
		d.QuoteIdentifier("instance_id"), d.Placeholder(1),
		d.QuoteIdentifier("id"), d.Placeholder(2),
		d.QuoteIdentifier("expires_at"), d.Placeholder(3),
	)

	r.qUpdateSession = fmt.Sprintf(
		"UPDATE zitadel_sessions SET %s = %s, %s = %s, %s = %s, %s = %s WHERE %s = %s AND %s = %s",
		d.QuoteIdentifier("user_agent"), d.Placeholder(1),
		d.QuoteIdentifier("ip_address"), d.Placeholder(2),
		d.QuoteIdentifier("data"), d.Placeholder(3),
		d.QuoteIdentifier("expires_at"), d.Placeholder(4),
		d.QuoteIdentifier("instance_id"), d.Placeholder(5),
		d.QuoteIdentifier("id"), d.Placeholder(6),
	)

	r.qDeleteSession = fmt.Sprintf(
		"DELETE FROM zitadel_sessions WHERE %s = %s AND %s = %s",
		d.QuoteIdentifier("instance_id"), d.Placeholder(1),
		d.QuoteIdentifier("id"), d.Placeholder(2),
	)

	r.qJanitor = fmt.Sprintf(
		"DELETE FROM zitadel_sessions WHERE %s < %s",
		d.QuoteIdentifier("expires_at"), d.Placeholder(1),
	)
}

func (r *SessionRepo) CreateSession(ctx context.Context, s *storage.Session) error {
	var dataJSON []byte
	if len(s.Data) > 0 {
		dataJSON, _ = json.Marshal(s.Data)
	} else {
		dataJSON = []byte("{}")
	}

	_, err := r.db.ExecContext(ctx, r.qCreateSession,
		s.ID, s.InstanceID, s.UserID, s.UserAgent, s.IPAddress, string(dataJSON), s.CreatedAt, s.ExpiresAt,
	)
	return r.dialect.TranslateError(err)
}

func (r *SessionRepo) GetSession(ctx context.Context, instanceID, sessionID string) (*storage.Session, error) {
	row := r.db.QueryRowContext(ctx, r.qGetSession, instanceID, sessionID, time.Now())
	s := &storage.Session{}
	var dataStr string
	err := row.Scan(&s.ID, &s.InstanceID, &s.UserID, &s.UserAgent, &s.IPAddress, &dataStr, &s.CreatedAt, &s.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, r.dialect.TranslateError(err)
	}
	if dataStr != "" && dataStr != "{}" {
		_ = json.Unmarshal([]byte(dataStr), &s.Data)
	}
	return s, nil
}

func (r *SessionRepo) UpdateSession(ctx context.Context, s *storage.Session) error {
	var dataJSON []byte
	if len(s.Data) > 0 {
		dataJSON, _ = json.Marshal(s.Data)
	} else {
		dataJSON = []byte("{}")
	}

	_, err := r.db.ExecContext(ctx, r.qUpdateSession, s.UserAgent, s.IPAddress, string(dataJSON), s.ExpiresAt, s.InstanceID, s.ID)
	return r.dialect.TranslateError(err)
}

func (r *SessionRepo) DeleteSession(ctx context.Context, instanceID, sessionID string) error {
	_, err := r.db.ExecContext(ctx, r.qDeleteSession, instanceID, sessionID)
	return r.dialect.TranslateError(err)
}

// startJanitor launches a background goroutine to asynchronously clean up expired rows
func (r *SessionRepo) startJanitor() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-r.ctx.Done():
				return
			case <-ticker.C:
				_, _ = r.db.ExecContext(context.Background(), r.qJanitor, time.Now())
			}
		}
	}()
}

// Close gracefully stops the janitor goroutine
func (r *SessionRepo) Close() error {
	if r.cancel != nil {
		r.cancel()
		r.wg.Wait()
	}
	return nil
}

// AuditRepo implements storage.AuditRepository using relational database with buffered batch inserter.
type AuditRepo struct {
	db           *sql.DB
	dialect      relational.Dialect
	buffer       chan *storage.AuditEntry
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	qInsertAudit string
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
	a.initQueries(dialect)
	a.startBatchWriter()
	return a
}

func (a *AuditRepo) initQueries(d relational.Dialect) {
	a.qInsertAudit = fmt.Sprintf(
		"INSERT INTO zitadel_audit_logs (%s, %s, %s, %s, %s, %s, %s, %s, %s) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)",
		d.QuoteIdentifier("id"),
		d.QuoteIdentifier("instance_id"),
		d.QuoteIdentifier("actor_id"),
		d.QuoteIdentifier("action"),
		d.QuoteIdentifier("resource_type"),
		d.QuoteIdentifier("resource_id"),
		d.QuoteIdentifier("status"),
		d.QuoteIdentifier("details"),
		d.QuoteIdentifier("created_at"),
		d.Placeholder(1),
		d.Placeholder(2),
		d.Placeholder(3),
		d.Placeholder(4),
		d.Placeholder(5),
		d.Placeholder(6),
		d.Placeholder(7),
		d.Placeholder(8),
		d.Placeholder(9),
	)
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
	var detailsJSON []byte
	if len(entry.Details) > 0 {
		detailsJSON, _ = json.Marshal(entry.Details)
	} else {
		detailsJSON = []byte("{}")
	}

	_, err := a.db.ExecContext(ctx, a.qInsertAudit,
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

			// High performance batch transaction commit
			tx, err := a.db.BeginTx(context.Background(), nil)
			if err != nil {
				for _, entry := range batch {
					_ = a.writeSingle(context.Background(), entry)
				}
				batch = batch[:0]
				return
			}

			stmt, err := tx.Prepare(a.qInsertAudit)
			if err != nil {
				_ = tx.Rollback()
				for _, entry := range batch {
					_ = a.writeSingle(context.Background(), entry)
				}
				batch = batch[:0]
				return
			}
			defer stmt.Close()

			for _, entry := range batch {
				var detailsJSON []byte
				if len(entry.Details) > 0 {
					detailsJSON, _ = json.Marshal(entry.Details)
				} else {
					detailsJSON = []byte("{}")
				}
				_, _ = stmt.Exec(entry.ID, entry.InstanceID, entry.ActorID, entry.Action, entry.ResourceType, entry.ResourceID, entry.Status, string(detailsJSON), entry.Timestamp)
			}
			_ = tx.Commit()
			batch = batch[:0]
		}

		for {
			select {
			case <-a.ctx.Done():
				// Drain buffer completely before stopping
			drain:
				for {
					select {
					case entry := <-a.buffer:
						batch = append(batch, entry)
						if len(batch) >= 100 {
							flush()
						}
					default:
						break drain
					}
				}
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

func (a *AuditRepo) Close() error {
	if a.cancel != nil {
		a.cancel()
		a.wg.Wait()
	}
	return nil
}

func (a *AuditRepo) Query(ctx context.Context, filter storage.AuditFilter) ([]*storage.AuditEntry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	args := []any{filter.InstanceID}
	whereClauses := []string{
		fmt.Sprintf("%s = %s", a.dialect.QuoteIdentifier("instance_id"), a.dialect.Placeholder(len(args))),
	}

	if filter.ActorID != "" {
		args = append(args, filter.ActorID)
		whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", a.dialect.QuoteIdentifier("actor_id"), a.dialect.Placeholder(len(args))))
	}
	if filter.ResourceType != "" {
		args = append(args, filter.ResourceType)
		whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", a.dialect.QuoteIdentifier("resource_type"), a.dialect.Placeholder(len(args))))
	}
	if filter.ResourceID != "" {
		args = append(args, filter.ResourceID)
		whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", a.dialect.QuoteIdentifier("resource_id"), a.dialect.Placeholder(len(args))))
	}
	if filter.FromTime != nil {
		args = append(args, *filter.FromTime)
		whereClauses = append(whereClauses, fmt.Sprintf("%s >= %s", a.dialect.QuoteIdentifier("created_at"), a.dialect.Placeholder(len(args))))
	}
	if filter.ToTime != nil {
		args = append(args, *filter.ToTime)
		whereClauses = append(whereClauses, fmt.Sprintf("%s <= %s", a.dialect.QuoteIdentifier("created_at"), a.dialect.Placeholder(len(args))))
	}

	whereStr := strings.Join(whereClauses, " AND ")
	query := fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s, %s, %s, %s FROM zitadel_audit_logs WHERE %s ORDER BY %s DESC %s",
		a.dialect.QuoteIdentifier("id"),
		a.dialect.QuoteIdentifier("instance_id"),
		a.dialect.QuoteIdentifier("actor_id"),
		a.dialect.QuoteIdentifier("action"),
		a.dialect.QuoteIdentifier("resource_type"),
		a.dialect.QuoteIdentifier("resource_id"),
		a.dialect.QuoteIdentifier("status"),
		a.dialect.QuoteIdentifier("details"),
		a.dialect.QuoteIdentifier("created_at"),
		whereStr,
		a.dialect.QuoteIdentifier("created_at"),
		a.dialect.PaginationClause(0, limit),
	)

	rows, err := a.db.QueryContext(ctx, query, args...)
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
		if detailsStr != "" && detailsStr != "{}" {
			_ = json.Unmarshal([]byte(detailsStr), &e.Details)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
