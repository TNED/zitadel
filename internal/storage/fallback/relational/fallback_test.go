//go:build !storage_dynamodb && !storage_firestore

package relational_fallback_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/zitadel/zitadel/internal/storage"
	relational_fallback "github.com/zitadel/zitadel/internal/storage/fallback/relational"
	"github.com/zitadel/zitadel/internal/storage/relational/postgres"
)

func TestSessionRepositoryCRUD(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	dialect := &postgres.Dialect{}
	repo := relational_fallback.NewSessionRepository(db, dialect)
	defer repo.Close()

	ctx := context.Background()

	// 1. CreateSession
	s := &storage.Session{
		ID:         "sess-1",
		InstanceID: "inst-1",
		UserID:     "usr-1",
		UserAgent:  "Mozilla/5.0",
		IPAddress:  "127.0.0.1",
		Data:       map[string]interface{}{"role": "admin"},
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(1 * time.Hour),
	}

	mock.ExpectExec("INSERT INTO zitadel_sessions").
		WithArgs(s.ID, s.InstanceID, s.UserID, s.UserAgent, s.IPAddress, sqlmock.AnyArg(), s.CreatedAt, s.ExpiresAt).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// 2. GetSession
	rows := sqlmock.NewRows([]string{
		"id", "instance_id", "user_id", "user_agent", "ip_address", "data", "created_at", "expires_at",
	}).AddRow("sess-1", "inst-1", "usr-1", "Mozilla/5.0", "127.0.0.1", `{"role":"admin"}`, s.CreatedAt, s.ExpiresAt)

	mock.ExpectQuery("SELECT .* FROM zitadel_sessions WHERE .*").
		WithArgs("inst-1", "sess-1", sqlmock.AnyArg()).
		WillReturnRows(rows)

	got, err := repo.GetSession(ctx, "inst-1", "sess-1")
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if got.UserID != "usr-1" {
		t.Errorf("expected user_id usr-1, got %s", got.UserID)
	}

	// 3. UpdateSession
	mock.ExpectExec("UPDATE zitadel_sessions SET .*").
		WithArgs("Chrome/100", "10.0.0.1", sqlmock.AnyArg(), s.ExpiresAt, "inst-1", "sess-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	s.UserAgent = "Chrome/100"
	s.IPAddress = "10.0.0.1"
	if err := repo.UpdateSession(ctx, s); err != nil {
		t.Fatalf("UpdateSession failed: %v", err)
	}

	// 4. DeleteSession
	mock.ExpectExec("DELETE FROM zitadel_sessions WHERE .*").
		WithArgs("inst-1", "sess-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.DeleteSession(ctx, "inst-1", "sess-1"); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestAuditRepositoryBatchAndQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	dialect := &postgres.Dialect{}
	repo := relational_fallback.NewAuditRepository(db, dialect)

	ctx := context.Background()
	now := time.Now()

	// 1. Log entry into buffer
	entry := &storage.AuditEntry{
		ID:           "aud-1",
		InstanceID:   "inst-1",
		ActorID:      "usr-1",
		Action:       "user.login",
		ResourceType: "user",
		ResourceID:   "usr-1",
		Status:       "SUCCESS",
		Details:      map[string]interface{}{"ip": "127.0.0.1"},
		Timestamp:    now,
	}

	if err := repo.Log(ctx, entry); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	// Close repo which flushes pending buffer in a single transaction with prepared statement
	mock.ExpectBegin()
	mock.ExpectPrepare("INSERT INTO zitadel_audit_logs")
	mock.ExpectExec("INSERT INTO zitadel_audit_logs").
		WithArgs(entry.ID, entry.InstanceID, entry.ActorID, entry.Action, entry.ResourceType, entry.ResourceID, entry.Status, sqlmock.AnyArg(), entry.Timestamp).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := repo.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// 2. Query with filters
	queryRows := sqlmock.NewRows([]string{
		"id", "instance_id", "actor_id", "action", "resource_type", "resource_id", "status", "details", "created_at",
	}).AddRow("aud-1", "inst-1", "usr-1", "user.login", "user", "usr-1", "SUCCESS", `{"ip":"127.0.0.1"}`, now)

	mock.ExpectQuery("SELECT .* FROM zitadel_audit_logs WHERE .*").
		WithArgs("inst-1", "usr-1", "user").
		WillReturnRows(queryRows)

	filter := storage.AuditFilter{
		InstanceID:   "inst-1",
		ActorID:      "usr-1",
		ResourceType: "user",
		Limit:        10,
	}

	results, err := repo.Query(ctx, filter)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "user.login" {
		t.Errorf("expected action user.login, got %s", results[0].Action)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}
