package migration_test

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/zitadel/zitadel/internal/storage"
	"github.com/zitadel/zitadel/internal/storage/migration"
)

func TestMigrationRunnerSQLite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}
	defer db.Close()

	// Expect all DDL statements to succeed
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS zitadel_schema_version")).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS zitadel_instances.*").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(".*").WillReturnResult(sqlmock.NewResult(0, 0))

	runner := migration.NewRunner(db, storage.DialectSQLite).SetPureRelational(false)
	ctx := context.Background()

	// The runner splits statements and executes them
	_ = runner.Run(ctx)
}
