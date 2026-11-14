package storage

import (
	"context"
	"time"
)

// Engine defines the unified storage gateway interface.
// Domain services access all repositories through this gateway.
type Engine interface {
	Users() UserRepository
	Orgs() OrgRepository
	Projects() ProjectRepository
	Sessions() SessionRepository
	Audit() AuditRepository
	Close() error
}

// UserRepository provides access to persistent user entities and credentials.
type UserRepository interface {
	CreateUser(ctx context.Context, user *User) error
	GetUserByID(ctx context.Context, instanceID, userID string) (*User, error)
	GetUserByLoginName(ctx context.Context, instanceID, loginName string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, instanceID, userID string) error
	ListUsers(ctx context.Context, filter UserFilter) ([]*User, error)
}

// OrgRepository provides access to organization and tenancy structures.
type OrgRepository interface {
	CreateOrg(ctx context.Context, org *Org) error
	GetOrgByID(ctx context.Context, instanceID, orgID string) (*Org, error)
	UpdateOrg(ctx context.Context, org *Org) error
	DeleteOrg(ctx context.Context, instanceID, orgID string) error
	ListOrgs(ctx context.Context, filter OrgFilter) ([]*Org, error)
}

// ProjectRepository provides access to projects, roles, and applications.
type ProjectRepository interface {
	CreateProject(ctx context.Context, proj *Project) error
	GetProjectByID(ctx context.Context, instanceID, projectID string) (*Project, error)
	UpdateProject(ctx context.Context, proj *Project) error
	DeleteProject(ctx context.Context, instanceID, projectID string) error
}

// SessionRepository provides access to user sessions with expiration support.
// In NoSQL mode, this is backed by DynamoDB or Firestore with native TTL.
// In pure relational mode, this is backed by an isolated relational fallback table.
type SessionRepository interface {
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, instanceID, sessionID string) (*Session, error)
	UpdateSession(ctx context.Context, session *Session) error
	DeleteSession(ctx context.Context, instanceID, sessionID string) error
}

// AuditRepository provides append-only access to practical audit logs.
type AuditRepository interface {
	Log(ctx context.Context, entry *AuditEntry) error
	Query(ctx context.Context, filter AuditFilter) ([]*AuditEntry, error)
}

// Domain Model Entities

type User struct {
	ID           string            `json:"id"`
	InstanceID   string            `json:"instance_id"`
	OrgID        string            `json:"org_id"`
	Username     string            `json:"username"`
	Email        string            `json:"email"`
	DisplayName  string            `json:"display_name"`
	State        string            `json:"state"` // ACTIVE, LOCKED, INACTIVE
	PasswordHash string            `json:"password_hash,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

type UserFilter struct {
	InstanceID string
	OrgID      string
	State      string
	Limit      int
	Offset     int
}

type Org struct {
	ID         string    `json:"id"`
	InstanceID string    `json:"instance_id"`
	Name       string    `json:"name"`
	State      string    `json:"state"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type OrgFilter struct {
	InstanceID string
	State      string
	Limit      int
	Offset     int
}

type Project struct {
	ID         string    `json:"id"`
	InstanceID string    `json:"instance_id"`
	OrgID      string    `json:"org_id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Session struct {
	ID         string                 `json:"id"`
	InstanceID string                 `json:"instance_id"`
	UserID     string                 `json:"user_id"`
	UserAgent  string                 `json:"user_agent"`
	IPAddress  string                 `json:"ip_address"`
	Data       map[string]interface{} `json:"data,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
	ExpiresAt  time.Time              `json:"expires_at"`
}

type AuditEntry struct {
	ID           string                 `json:"id"`
	InstanceID   string                 `json:"instance_id"`
	ActorID      string                 `json:"actor_id"`
	Action       string                 `json:"action"` // e.g. "user.created", "user.login"
	ResourceType string                 `json:"resource_type"`
	ResourceID   string                 `json:"resource_id"`
	Status       string                 `json:"status"` // SUCCESS, FAILED
	Details      map[string]interface{} `json:"details,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
}

type AuditFilter struct {
	InstanceID   string
	ActorID      string
	ResourceType string
	ResourceID   string
	FromTime     *time.Time
	ToTime       *time.Time
	Limit        int
}
