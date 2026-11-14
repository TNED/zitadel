package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zitadel/zitadel/internal/storage"
	"github.com/zitadel/zitadel/internal/storage/relational"
)

// Repository implements core relational entities storage.
// Strictly manages Users, Organizations, and Projects without session/audit pollution.
type Repository struct {
	db      *sql.DB
	dialect relational.Dialect
}

// NewRepository creates a new core relational repository.
func NewRepository(db *sql.DB, dialect relational.Dialect) *Repository {
	return &Repository{
		db:      db,
		dialect: dialect,
	}
}

// --- UserRepository Implementation ---

func (r *Repository) CreateUser(ctx context.Context, u *storage.User) error {
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	u.UpdatedAt = u.CreatedAt

	metaJSON, _ := json.Marshal(u.Metadata)
	query := fmt.Sprintf(
		"INSERT INTO zitadel_users (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)",
		r.dialect.QuoteIdentifier("id"),
		r.dialect.QuoteIdentifier("instance_id"),
		r.dialect.QuoteIdentifier("org_id"),
		r.dialect.QuoteIdentifier("username"),
		r.dialect.QuoteIdentifier("email"),
		r.dialect.QuoteIdentifier("display_name"),
		r.dialect.QuoteIdentifier("state"),
		r.dialect.QuoteIdentifier("password_hash"),
		r.dialect.QuoteIdentifier("metadata"),
		r.dialect.QuoteIdentifier("created_at"),
		r.dialect.QuoteIdentifier("updated_at"),
		r.dialect.Placeholder(1),
		r.dialect.Placeholder(2),
		r.dialect.Placeholder(3),
		r.dialect.Placeholder(4),
		r.dialect.Placeholder(5),
		r.dialect.Placeholder(6),
		r.dialect.Placeholder(7),
		r.dialect.Placeholder(8),
		r.dialect.Placeholder(9),
		r.dialect.Placeholder(10),
		r.dialect.Placeholder(11),
	)

	_, err := r.db.ExecContext(ctx, query,
		u.ID, u.InstanceID, u.OrgID, u.Username, u.Email, u.DisplayName, u.State, u.PasswordHash, string(metaJSON), u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return r.dialect.TranslateError(err)
	}
	return nil
}

func (r *Repository) GetUserByID(ctx context.Context, instanceID, userID string) (*storage.User, error) {
	query := fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s FROM zitadel_users WHERE %s = %s AND %s = %s",
		r.dialect.QuoteIdentifier("id"),
		r.dialect.QuoteIdentifier("instance_id"),
		r.dialect.QuoteIdentifier("org_id"),
		r.dialect.QuoteIdentifier("username"),
		r.dialect.QuoteIdentifier("email"),
		r.dialect.QuoteIdentifier("display_name"),
		r.dialect.QuoteIdentifier("state"),
		r.dialect.QuoteIdentifier("password_hash"),
		r.dialect.QuoteIdentifier("metadata"),
		r.dialect.QuoteIdentifier("created_at"),
		r.dialect.QuoteIdentifier("updated_at"),
		r.dialect.QuoteIdentifier("instance_id"), r.dialect.Placeholder(1),
		r.dialect.QuoteIdentifier("id"), r.dialect.Placeholder(2),
	)

	row := r.db.QueryRowContext(ctx, query, instanceID, userID)
	u := &storage.User{}
	var metaStr string
	err := row.Scan(&u.ID, &u.InstanceID, &u.OrgID, &u.Username, &u.Email, &u.DisplayName, &u.State, &u.PasswordHash, &metaStr, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, relational.ErrNotFound
	}
	if err != nil {
		return nil, r.dialect.TranslateError(err)
	}
	if metaStr != "" {
		_ = json.Unmarshal([]byte(metaStr), &u.Metadata)
	}
	return u, nil
}

func (r *Repository) GetUserByLoginName(ctx context.Context, instanceID, loginName string) (*storage.User, error) {
	query := fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s FROM zitadel_users WHERE %s = %s AND (%s = %s OR %s = %s)",
		r.dialect.QuoteIdentifier("id"),
		r.dialect.QuoteIdentifier("instance_id"),
		r.dialect.QuoteIdentifier("org_id"),
		r.dialect.QuoteIdentifier("username"),
		r.dialect.QuoteIdentifier("email"),
		r.dialect.QuoteIdentifier("display_name"),
		r.dialect.QuoteIdentifier("state"),
		r.dialect.QuoteIdentifier("password_hash"),
		r.dialect.QuoteIdentifier("metadata"),
		r.dialect.QuoteIdentifier("created_at"),
		r.dialect.QuoteIdentifier("updated_at"),
		r.dialect.QuoteIdentifier("instance_id"), r.dialect.Placeholder(1),
		r.dialect.QuoteIdentifier("username"), r.dialect.Placeholder(2),
		r.dialect.QuoteIdentifier("email"), r.dialect.Placeholder(3),
	)

	row := r.db.QueryRowContext(ctx, query, instanceID, loginName, loginName)
	u := &storage.User{}
	var metaStr string
	err := row.Scan(&u.ID, &u.InstanceID, &u.OrgID, &u.Username, &u.Email, &u.DisplayName, &u.State, &u.PasswordHash, &metaStr, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, relational.ErrNotFound
	}
	if err != nil {
		return nil, r.dialect.TranslateError(err)
	}
	if metaStr != "" {
		_ = json.Unmarshal([]byte(metaStr), &u.Metadata)
	}
	return u, nil
}

func (r *Repository) UpdateUser(ctx context.Context, u *storage.User) error {
	u.UpdatedAt = time.Now()
	metaJSON, _ := json.Marshal(u.Metadata)

	query := fmt.Sprintf(
		"UPDATE zitadel_users SET %s = %s, %s = %s, %s = %s, %s = %s, %s = %s, %s = %s, %s = %s WHERE %s = %s AND %s = %s",
		r.dialect.QuoteIdentifier("username"), r.dialect.Placeholder(1),
		r.dialect.QuoteIdentifier("email"), r.dialect.Placeholder(2),
		r.dialect.QuoteIdentifier("display_name"), r.dialect.Placeholder(3),
		r.dialect.QuoteIdentifier("state"), r.dialect.Placeholder(4),
		r.dialect.QuoteIdentifier("password_hash"), r.dialect.Placeholder(5),
		r.dialect.QuoteIdentifier("metadata"), r.dialect.Placeholder(6),
		r.dialect.QuoteIdentifier("updated_at"), r.dialect.Placeholder(7),
		r.dialect.QuoteIdentifier("instance_id"), r.dialect.Placeholder(8),
		r.dialect.QuoteIdentifier("id"), r.dialect.Placeholder(9),
	)

	_, err := r.db.ExecContext(ctx, query,
		u.Username, u.Email, u.DisplayName, u.State, u.PasswordHash, string(metaJSON), u.UpdatedAt, u.InstanceID, u.ID,
	)
	if err != nil {
		return r.dialect.TranslateError(err)
	}
	return nil
}

func (r *Repository) DeleteUser(ctx context.Context, instanceID, userID string) error {
	query := fmt.Sprintf(
		"DELETE FROM zitadel_users WHERE %s = %s AND %s = %s",
		r.dialect.QuoteIdentifier("instance_id"), r.dialect.Placeholder(1),
		r.dialect.QuoteIdentifier("id"), r.dialect.Placeholder(2),
	)
	_, err := r.db.ExecContext(ctx, query, instanceID, userID)
	return r.dialect.TranslateError(err)
}

func (r *Repository) ListUsers(ctx context.Context, filter storage.UserFilter) ([]*storage.User, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	query := fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s FROM zitadel_users WHERE %s = %s %s",
		r.dialect.QuoteIdentifier("id"),
		r.dialect.QuoteIdentifier("instance_id"),
		r.dialect.QuoteIdentifier("org_id"),
		r.dialect.QuoteIdentifier("username"),
		r.dialect.QuoteIdentifier("email"),
		r.dialect.QuoteIdentifier("display_name"),
		r.dialect.QuoteIdentifier("state"),
		r.dialect.QuoteIdentifier("password_hash"),
		r.dialect.QuoteIdentifier("metadata"),
		r.dialect.QuoteIdentifier("created_at"),
		r.dialect.QuoteIdentifier("updated_at"),
		r.dialect.QuoteIdentifier("instance_id"), r.dialect.Placeholder(1),
		r.dialect.PaginationClause(filter.Offset, limit),
	)

	rows, err := r.db.QueryContext(ctx, query, filter.InstanceID)
	if err != nil {
		return nil, r.dialect.TranslateError(err)
	}
	defer rows.Close()

	var users []*storage.User
	for rows.Next() {
		u := &storage.User{}
		var metaStr string
		if err := rows.Scan(&u.ID, &u.InstanceID, &u.OrgID, &u.Username, &u.Email, &u.DisplayName, &u.State, &u.PasswordHash, &metaStr, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		if metaStr != "" {
			_ = json.Unmarshal([]byte(metaStr), &u.Metadata)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// --- OrgRepository Implementation ---

func (r *Repository) CreateOrg(ctx context.Context, o *storage.Org) error {
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now()
	}
	o.UpdatedAt = o.CreatedAt

	query := fmt.Sprintf(
		"INSERT INTO zitadel_orgs (%s, %s, %s, %s, %s, %s) VALUES (%s, %s, %s, %s, %s, %s)",
		r.dialect.QuoteIdentifier("id"),
		r.dialect.QuoteIdentifier("instance_id"),
		r.dialect.QuoteIdentifier("name"),
		r.dialect.QuoteIdentifier("state"),
		r.dialect.QuoteIdentifier("created_at"),
		r.dialect.QuoteIdentifier("updated_at"),
		r.dialect.Placeholder(1),
		r.dialect.Placeholder(2),
		r.dialect.Placeholder(3),
		r.dialect.Placeholder(4),
		r.dialect.Placeholder(5),
		r.dialect.Placeholder(6),
	)
	_, err := r.db.ExecContext(ctx, query, o.ID, o.InstanceID, o.Name, o.State, o.CreatedAt, o.UpdatedAt)
	return r.dialect.TranslateError(err)
}

func (r *Repository) GetOrgByID(ctx context.Context, instanceID, orgID string) (*storage.Org, error) {
	query := fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s FROM zitadel_orgs WHERE %s = %s AND %s = %s",
		r.dialect.QuoteIdentifier("id"),
		r.dialect.QuoteIdentifier("instance_id"),
		r.dialect.QuoteIdentifier("name"),
		r.dialect.QuoteIdentifier("state"),
		r.dialect.QuoteIdentifier("created_at"),
		r.dialect.QuoteIdentifier("updated_at"),
		r.dialect.QuoteIdentifier("instance_id"), r.dialect.Placeholder(1),
		r.dialect.QuoteIdentifier("id"), r.dialect.Placeholder(2),
	)
	row := r.db.QueryRowContext(ctx, query, instanceID, orgID)
	o := &storage.Org{}
	err := row.Scan(&o.ID, &o.InstanceID, &o.Name, &o.State, &o.CreatedAt, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, relational.ErrNotFound
	}
	return o, r.dialect.TranslateError(err)
}

func (r *Repository) UpdateOrg(ctx context.Context, o *storage.Org) error {
	o.UpdatedAt = time.Now()
	query := fmt.Sprintf(
		"UPDATE zitadel_orgs SET %s = %s, %s = %s, %s = %s WHERE %s = %s AND %s = %s",
		r.dialect.QuoteIdentifier("name"), r.dialect.Placeholder(1),
		r.dialect.QuoteIdentifier("state"), r.dialect.Placeholder(2),
		r.dialect.QuoteIdentifier("updated_at"), r.dialect.Placeholder(3),
		r.dialect.QuoteIdentifier("instance_id"), r.dialect.Placeholder(4),
		r.dialect.QuoteIdentifier("id"), r.dialect.Placeholder(5),
	)
	_, err := r.db.ExecContext(ctx, query, o.Name, o.State, o.UpdatedAt, o.InstanceID, o.ID)
	return r.dialect.TranslateError(err)
}

func (r *Repository) DeleteOrg(ctx context.Context, instanceID, orgID string) error {
	query := fmt.Sprintf(
		"DELETE FROM zitadel_orgs WHERE %s = %s AND %s = %s",
		r.dialect.QuoteIdentifier("instance_id"), r.dialect.Placeholder(1),
		r.dialect.QuoteIdentifier("id"), r.dialect.Placeholder(2),
	)
	_, err := r.db.ExecContext(ctx, query, instanceID, orgID)
	return r.dialect.TranslateError(err)
}

func (r *Repository) ListOrgs(ctx context.Context, filter storage.OrgFilter) ([]*storage.Org, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	query := fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s FROM zitadel_orgs WHERE %s = %s %s",
		r.dialect.QuoteIdentifier("id"),
		r.dialect.QuoteIdentifier("instance_id"),
		r.dialect.QuoteIdentifier("name"),
		r.dialect.QuoteIdentifier("state"),
		r.dialect.QuoteIdentifier("created_at"),
		r.dialect.QuoteIdentifier("updated_at"),
		r.dialect.QuoteIdentifier("instance_id"), r.dialect.Placeholder(1),
		r.dialect.PaginationClause(filter.Offset, limit),
	)
	rows, err := r.db.QueryContext(ctx, query, filter.InstanceID)
	if err != nil {
		return nil, r.dialect.TranslateError(err)
	}
	defer rows.Close()

	var orgs []*storage.Org
	for rows.Next() {
		o := &storage.Org{}
		if err := rows.Scan(&o.ID, &o.InstanceID, &o.Name, &o.State, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		orgs = append(orgs, o)
	}
	return orgs, rows.Err()
}

// --- ProjectRepository Implementation ---

func (r *Repository) CreateProject(ctx context.Context, p *storage.Project) error {
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	p.UpdatedAt = p.CreatedAt

	query := fmt.Sprintf(
		"INSERT INTO zitadel_projects (%s, %s, %s, %s, %s, %s) VALUES (%s, %s, %s, %s, %s, %s)",
		r.dialect.QuoteIdentifier("id"),
		r.dialect.QuoteIdentifier("instance_id"),
		r.dialect.QuoteIdentifier("org_id"),
		r.dialect.QuoteIdentifier("name"),
		r.dialect.QuoteIdentifier("created_at"),
		r.dialect.QuoteIdentifier("updated_at"),
		r.dialect.Placeholder(1),
		r.dialect.Placeholder(2),
		r.dialect.Placeholder(3),
		r.dialect.Placeholder(4),
		r.dialect.Placeholder(5),
		r.dialect.Placeholder(6),
	)
	_, err := r.db.ExecContext(ctx, query, p.ID, p.InstanceID, p.OrgID, p.Name, p.CreatedAt, p.UpdatedAt)
	return r.dialect.TranslateError(err)
}

func (r *Repository) GetProjectByID(ctx context.Context, instanceID, projectID string) (*storage.Project, error) {
	query := fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s FROM zitadel_projects WHERE %s = %s AND %s = %s",
		r.dialect.QuoteIdentifier("id"),
		r.dialect.QuoteIdentifier("instance_id"),
		r.dialect.QuoteIdentifier("org_id"),
		r.dialect.QuoteIdentifier("name"),
		r.dialect.QuoteIdentifier("created_at"),
		r.dialect.QuoteIdentifier("updated_at"),
		r.dialect.QuoteIdentifier("instance_id"), r.dialect.Placeholder(1),
		r.dialect.QuoteIdentifier("id"), r.dialect.Placeholder(2),
	)
	row := r.db.QueryRowContext(ctx, query, instanceID, projectID)
	p := &storage.Project{}
	err := row.Scan(&p.ID, &p.InstanceID, &p.OrgID, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, relational.ErrNotFound
	}
	return p, r.dialect.TranslateError(err)
}

func (r *Repository) UpdateProject(ctx context.Context, p *storage.Project) error {
	p.UpdatedAt = time.Now()
	query := fmt.Sprintf(
		"UPDATE zitadel_projects SET %s = %s, %s = %s WHERE %s = %s AND %s = %s",
		r.dialect.QuoteIdentifier("name"), r.dialect.Placeholder(1),
		r.dialect.QuoteIdentifier("updated_at"), r.dialect.Placeholder(2),
		r.dialect.QuoteIdentifier("instance_id"), r.dialect.Placeholder(3),
		r.dialect.QuoteIdentifier("id"), r.dialect.Placeholder(4),
	)
	_, err := r.db.ExecContext(ctx, query, p.Name, p.UpdatedAt, p.InstanceID, p.ID)
	return r.dialect.TranslateError(err)
}

func (r *Repository) DeleteProject(ctx context.Context, instanceID, projectID string) error {
	query := fmt.Sprintf(
		"DELETE FROM zitadel_projects WHERE %s = %s AND %s = %s",
		r.dialect.QuoteIdentifier("instance_id"), r.dialect.Placeholder(1),
		r.dialect.QuoteIdentifier("id"), r.dialect.Placeholder(2),
	)
	_, err := r.db.ExecContext(ctx, query, instanceID, projectID)
	return r.dialect.TranslateError(err)
}
