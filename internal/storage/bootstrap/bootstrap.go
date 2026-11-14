package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/zitadel/zitadel/internal/storage"
)

// Config defines the declarative initialization parameters for a fresh database.
type Config struct {
	InstanceID     string
	InstanceName   string
	DefaultDomain  string
	OrgID          string
	OrgName        string
	AdminUsername  string
	AdminEmail     string
	AdminPassword  string
	ProjectID      string
	ProjectName    string
}

// DefaultConfig returns default initial parameters.
func DefaultConfig() Config {
	return Config{
		InstanceID:    "zitadel",
		InstanceName:  "ZITADEL Default Instance",
		DefaultDomain: "localhost",
		OrgID:         "org-default",
		OrgName:       "Default Organization",
		AdminUsername: "zitadel-admin",
		AdminEmail:    "admin@zitadel.local",
		AdminPassword: "Password123!",
		ProjectID:     "project-default",
		ProjectName:   "Management Project",
	}
}

// Bootstrapper handles idempotent first-time instance creation.
type Bootstrapper struct {
	db      *sql.DB
	dialect storage.Dialect
	config  Config
}

// NewBootstrapper creates a new bootstrapper.
func NewBootstrapper(db *sql.DB, dialect storage.Dialect, config Config) *Bootstrapper {
	if config.InstanceID == "" {
		config = DefaultConfig()
	}
	return &Bootstrapper{
		db:      db,
		dialect: dialect,
		config:  config,
	}
}

// EnsureInitialized checks if an instance exists. If empty, seeds the initial instance and admin.
func (b *Bootstrapper) EnsureInitialized(ctx context.Context) error {
	var count int
	err := b.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM zitadel_instances").Scan(&count)
	if err != nil {
		return fmt.Errorf("checking instances existence failed: %w", err)
	}

	if count > 0 {
		// Already initialized
		return nil
	}

	// Begin transaction
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning bootstrap tx failed: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// 1. Insert Default Instance
	insertInst := b.formatQuery("INSERT INTO zitadel_instances (id, name, default_domain, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)")
	if _, err := tx.ExecContext(ctx, insertInst, b.config.InstanceID, b.config.InstanceName, b.config.DefaultDomain, now, now); err != nil {
		return fmt.Errorf("bootstrapping default instance failed: %w", err)
	}

	// 2. Insert Default Org
	insertOrg := b.formatQuery("INSERT INTO zitadel_orgs (id, instance_id, name, primary_domain, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)")
	if _, err := tx.ExecContext(ctx, insertOrg, b.config.OrgID, b.config.InstanceID, b.config.OrgName, b.config.DefaultDomain, now, now); err != nil {
		return fmt.Errorf("bootstrapping default org failed: %w", err)
	}

	// 3. Insert Default Project
	insertProj := b.formatQuery("INSERT INTO zitadel_projects (id, instance_id, org_id, name, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6)")
	if _, err := tx.ExecContext(ctx, insertProj, b.config.ProjectID, b.config.InstanceID, b.config.OrgID, b.config.ProjectName, now, now); err != nil {
		return fmt.Errorf("bootstrapping default project failed: %w", err)
	}

	// 4. Insert Default Admin User
	adminUserID := "user-admin"
	insertUser := b.formatQuery(`INSERT INTO zitadel_users 
		(id, instance_id, org_id, username, email, is_email_verified, display_name, user_type, state, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`)
	if _, err := tx.ExecContext(ctx, insertUser,
		adminUserID, b.config.InstanceID, b.config.OrgID,
		b.config.AdminUsername, b.config.AdminEmail, true,
		"ZITADEL Administrator", "human", "active", now, now,
	); err != nil {
		return fmt.Errorf("bootstrapping admin user failed: %w", err)
	}

	// 5. Hash password and insert credential
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(b.config.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing initial admin password failed: %w", err)
	}

	insertCred := b.formatQuery("INSERT INTO zitadel_credentials (id, user_id, instance_id, cred_type, secret_hash, metadata, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)")
	if _, err := tx.ExecContext(ctx, insertCred, "cred-admin-pwd", adminUserID, b.config.InstanceID, "password", string(hashedPassword), "{}", now, now); err != nil {
		return fmt.Errorf("bootstrapping admin credential failed: %w", err)
	}

	return tx.Commit()
}

func (b *Bootstrapper) formatQuery(q string) string {
	if b.dialect == storage.DialectMySQL || b.dialect == storage.DialectSQLite {
		// Replace $1, $2 with ?
		for i := 15; i >= 1; i-- {
			q = replacePlaceholder(q, fmt.Sprintf("$%d", i), "?")
		}
	} else if b.dialect == storage.DialectMSSQL {
		for i := 15; i >= 1; i-- {
			q = replacePlaceholder(q, fmt.Sprintf("$%d", i), fmt.Sprintf("@p%d", i))
		}
	} else if b.dialect == storage.DialectOracle {
		for i := 15; i >= 1; i-- {
			q = replacePlaceholder(q, fmt.Sprintf("$%d", i), fmt.Sprintf(":%d", i))
		}
	}
	return q
}

func replacePlaceholder(s, old, newStr string) string {
	for {
		idx := findPlaceholder(s, old)
		if idx == -1 {
			break
		}
		s = s[:idx] + newStr + s[idx+len(old):]
	}
	return s
}

func findPlaceholder(s, target string) int {
	idx := 0
	for {
		pos := len(s[idx:])
		subIdx := -1
		for i := 0; i+len(target) <= len(s[idx:]); i++ {
			if s[idx+i:idx+i+len(target)] == target {
				subIdx = i
				break
			}
		}
		if subIdx == -1 {
			return -1
		}
		actualPos := idx + subIdx
		// Ensure next char is not a digit
		afterPos := actualPos + len(target)
		if afterPos < len(s) && s[afterPos] >= '0' && s[afterPos] <= '9' {
			idx = afterPos
			continue
		}
		return actualPos
	}
}
