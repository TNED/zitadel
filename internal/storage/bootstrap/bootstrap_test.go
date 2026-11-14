package bootstrap_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/zitadel/zitadel/internal/storage"
	"github.com/zitadel/zitadel/internal/storage/bootstrap"
)

func TestBootstrapperAlreadyInitialized(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	// Return 1 instance existing
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM zitadel_instances").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	boot := bootstrap.NewBootstrapper(db, storage.DialectPostgres, bootstrap.DefaultConfig())
	err = boot.EnsureInitialized(context.Background())
	if err != nil {
		t.Fatalf("expected nil error when already initialized, got %v", err)
	}
}

func TestBootstrapperSeedsDatabase(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	// Return 0 instances existing
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM zitadel_instances").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO zitadel_instances .*").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO zitadel_orgs .*").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO zitadel_projects .*").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO zitadel_users .*").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO zitadel_credentials .*").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	boot := bootstrap.NewBootstrapper(db, storage.DialectPostgres, bootstrap.DefaultConfig())
	err = boot.EnsureInitialized(context.Background())
	if err != nil {
		t.Fatalf("expected nil error on bootstrap, got %v", err)
	}
}

func TestBootstrapperSeedsOracle26ai(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	// Return 0 instances existing
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM zitadel_instances").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO zitadel_instances .*").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO zitadel_orgs .*").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO zitadel_projects .*").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO zitadel_users .*").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO zitadel_credentials .*").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	boot := bootstrap.NewBootstrapper(db, storage.DialectOracle26ai, bootstrap.DefaultConfig())
	err = boot.EnsureInitialized(context.Background())
	if err != nil {
		t.Fatalf("expected nil error on Oracle 26ai bootstrap, got %v", err)
	}
}
