package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/zitadel/zitadel/internal/storage"
	"github.com/zitadel/zitadel/internal/storage/relational"
	"github.com/zitadel/zitadel/internal/storage/relational/core"
	"github.com/zitadel/zitadel/internal/storage/relational/mysql"
	"github.com/zitadel/zitadel/internal/storage/relational/postgres"
	"github.com/zitadel/zitadel/internal/storage/relational/sqlite"
)

func TestDialectPlaceholdersAndQuotes(t *testing.T) {
	pg := &postgres.Dialect{}
	if pg.Placeholder(1) != "$1" {
		t.Errorf("expected $1, got %s", pg.Placeholder(1))
	}
	if pg.QuoteIdentifier("users") != `"users"` {
		t.Errorf("expected \"users\", got %s", pg.QuoteIdentifier("users"))
	}

	my := &mysql.Dialect{}
	if my.Placeholder(1) != "?" {
		t.Errorf("expected ?, got %s", my.Placeholder(1))
	}
	if my.QuoteIdentifier("users") != "`users`" {
		t.Errorf("expected `users`, got %s", my.QuoteIdentifier("users"))
	}

	sq := &sqlite.Dialect{}
	if sq.Placeholder(1) != "?" {
		t.Errorf("expected ?, got %s", sq.Placeholder(1))
	}
	if sq.QuoteIdentifier("users") != `"users"` {
		t.Errorf("expected \"users\", got %s", sq.QuoteIdentifier("users"))
	}
}

func TestCoreRepositoryUserCRUD(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	dialect := &postgres.Dialect{}
	repo := core.NewRepository(db, dialect)
	ctx := context.Background()

	// 1. Create User
	u := &storage.User{
		ID:          "usr-1",
		InstanceID:  "inst-1",
		OrgID:       "org-1",
		Username:    "alice",
		Email:       "alice@example.com",
		DisplayName: "Alice",
		State:       "ACTIVE",
		CreatedAt:   time.Now(),
	}

	mock.ExpectExec("INSERT INTO zitadel_users").
		WithArgs(u.ID, u.InstanceID, u.OrgID, u.Username, u.Email, u.DisplayName, u.State, u.PasswordHash, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// 2. Get User By ID
	rows := sqlmock.NewRows([]string{
		"id", "instance_id", "org_id", "username", "email", "display_name", "state", "password_hash", "metadata", "created_at", "updated_at",
	}).AddRow("usr-1", "inst-1", "org-1", "alice", "alice@example.com", "Alice", "ACTIVE", "", "{}", u.CreatedAt, u.CreatedAt)

	mock.ExpectQuery("SELECT .* FROM zitadel_users WHERE .*").
		WithArgs("inst-1", "usr-1").
		WillReturnRows(rows)

	got, err := repo.GetUserByID(ctx, "inst-1", "usr-1")
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if got.Username != "alice" {
		t.Errorf("expected username alice, got %s", got.Username)
	}

	// 3. Delete User
	mock.ExpectExec("DELETE FROM zitadel_users WHERE .*").
		WithArgs("inst-1", "usr-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.DeleteUser(ctx, "inst-1", "usr-1"); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestCoreRepositoryOrgCRUD(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	dialect := &mysql.Dialect{}
	repo := core.NewRepository(db, dialect)
	ctx := context.Background()

	o := &storage.Org{
		ID:         "org-1",
		InstanceID: "inst-1",
		Name:       "Acme Corp",
		State:      "ACTIVE",
		CreatedAt:  time.Now(),
	}

	mock.ExpectExec("INSERT INTO zitadel_orgs").
		WithArgs(o.ID, o.InstanceID, o.Name, o.State, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.CreateOrg(ctx, o); err != nil {
		t.Fatalf("CreateOrg failed: %v", err)
	}

	rows := sqlmock.NewRows([]string{
		"id", "instance_id", "name", "state", "created_at", "updated_at",
	}).AddRow("org-1", "inst-1", "Acme Corp", "ACTIVE", o.CreatedAt, o.CreatedAt)

	mock.ExpectQuery("SELECT .* FROM zitadel_orgs WHERE .*").
		WithArgs("inst-1", "org-1").
		WillReturnRows(rows)

	got, err := repo.GetOrgByID(ctx, "inst-1", "org-1")
	if err != nil {
		t.Fatalf("GetOrgByID failed: %v", err)
	}
	if got.Name != "Acme Corp" {
		t.Errorf("expected name Acme Corp, got %s", got.Name)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
