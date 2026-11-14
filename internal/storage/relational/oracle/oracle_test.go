//go:build storage_all || storage_oracle

package oracle_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/zitadel/zitadel/internal/storage/relational"
	"github.com/zitadel/zitadel/internal/storage/relational/oracle"
)

func TestProviderRegistration(t *testing.T) {
	for _, name := range []string{"oracle", "oracle23ai", "oracle26ai"} {
		p, ok := relational.GetProvider(name)
		if !ok {
			t.Errorf("expected provider %q to be registered", name)
		}
		if p == nil {
			t.Errorf("provider %q is nil", name)
		}
	}
}

func TestBuildDSN(t *testing.T) {
	// 1. Explicit DSN
	explicit := relational.Config{
		DSN: "oracle://user:pass@remote:1521/XEPDB1",
	}
	if got := oracle.BuildDSN(explicit); got != explicit.DSN {
		t.Errorf("expected %s, got %s", explicit.DSN, got)
	}

	// 2. Default host/port/db
	defaults := relational.Config{
		User:     "zitadel_user",
		Password: "Password123!",
	}
	gotDefault := oracle.BuildDSN(defaults)
	if !strings.Contains(gotDefault, "oracle://zitadel_user:Password123%21@localhost:1521/FREEPDB1") {
		t.Errorf("expected URL encoded password and default host/port, got %s", gotDefault)
	}

	// 3. OCI ATP 26ai with Wallet & SSL Options
	atpCfg := relational.Config{
		Host:     "adb.ap-singapore-1.oraclecloud.com",
		Port:     1522,
		Database: "myatp26ai_high",
		User:     "admin",
		Password: "SecretPassword!",
		Options: map[string]string{
			"SSL":    "enable",
			"WALLET": "/etc/oracle/wallet",
		},
	}
	gotATP := oracle.BuildDSN(atpCfg)
	if !strings.HasPrefix(gotATP, "oracle://admin:SecretPassword%21@adb.ap-singapore-1.oraclecloud.com:1522/myatp26ai_high?") {
		t.Errorf("expected ATP prefix, got %s", gotATP)
	}
	if !strings.Contains(gotATP, "SSL=enable") || !strings.Contains(gotATP, "WALLET=%2Fetc%2Foracle%2Fwallet") {
		t.Errorf("expected options in DSN query params, got %s", gotATP)
	}
}

func TestDialect(t *testing.T) {
	p, ok := relational.GetProvider("oracle26ai")
	if !ok {
		t.Fatal("oracle26ai provider not found")
	}
	d := p.Dialect()

	if d.Name() != "oracle26ai" {
		t.Errorf("expected dialect name oracle26ai, got %s", d.Name())
	}

	if q := d.QuoteIdentifier("zitadel_users"); q != `"ZITADEL_USERS"` {
		t.Errorf("expected \"ZITADEL_USERS\", got %s", q)
	}

	if ph := d.Placeholder(1); ph != ":1" {
		t.Errorf("expected :1, got %s", ph)
	}
	if ph := d.Placeholder(10); ph != ":10" {
		t.Errorf("expected :10, got %s", ph)
	}

	if pg := d.PaginationClause(0, 20); pg != "OFFSET 0 ROWS FETCH NEXT 20 ROWS ONLY" {
		t.Errorf("unexpected pagination: %s", pg)
	}
	if pg := d.PaginationClause(-5, 50); pg != "OFFSET 0 ROWS FETCH NEXT 50 ROWS ONLY" {
		t.Errorf("unexpected pagination with negative offset: %s", pg)
	}

	// Error translation
	ora00001 := errors.New("ORA-00001: unique constraint (SYSTEM.UQ_INSTANCE_ORG) violated")
	if err := d.TranslateError(ora00001); !errors.Is(err, relational.ErrDuplicateKey) {
		t.Errorf("expected ErrDuplicateKey, got %v", err)
	}

	otherErr := errors.New("ORA-00942: table or view does not exist")
	if err := d.TranslateError(otherErr); err != otherErr {
		t.Errorf("expected unchanged error, got %v", err)
	}

	if err := d.TranslateError(nil); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}
