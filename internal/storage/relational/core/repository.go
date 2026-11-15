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
// All static SQL query templates are pre-compiled and cached on creation to eliminate hot-path string allocation.
type Repository struct {
	db      *sql.DB
	dialect relational.Dialect

	// Pre-compiled SQL query strings
	qCreateUser         string
	qGetUserByID        string
	qGetUserByLoginName string
	qUpdateUser         string
	qDeleteUser         string
	qListUsersPrefix    string

	qCreateOrg      string
	qGetOrgByID     string
	qUpdateOrg      string
	qDeleteOrg      string
	qListOrgsPrefix string

	qCreateProject  string
	qGetProjectByID string
	qUpdateProject  string
	qDeleteProject  string
}

// NewRepository creates a new core relational repository with pre-computed query statements.
func NewRepository(db *sql.DB, dialect relational.Dialect) *Repository {
	r := &Repository{
		db:      db,
		dialect: dialect,
	}
	r.initQueries(dialect)
	return r
}

func (r *Repository) initQueries(d relational.Dialect) {
	// 1. Users Queries
	r.qCreateUser = fmt.Sprintf(
		"INSERT INTO zitadel_users (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s)",
		d.QuoteIdentifier("id"),
		d.QuoteIdentifier("instance_id"),
		d.QuoteIdentifier("org_id"),
		d.QuoteIdentifier("username"),
		d.QuoteIdentifier("email"),
		d.QuoteIdentifier("display_name"),
		d.QuoteIdentifier("state"),
		d.QuoteIdentifier("password_hash"),
		d.QuoteIdentifier("metadata"),
		d.QuoteIdentifier("created_at"),
		d.QuoteIdentifier("updated_at"),
		d.Placeholder(1),
		d.Placeholder(2),
		d.Placeholder(3),
		d.Placeholder(4),
		d.Placeholder(5),
		d.Placeholder(6),
		d.Placeholder(7),
		d.Placeholder(8),
		d.Placeholder(9),
		d.Placeholder(10),
		d.Placeholder(11),
	)

	r.qGetUserByID = fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s FROM zitadel_users WHERE %s = %s AND %s = %s",
		d.QuoteIdentifier("id"),
		d.QuoteIdentifier("instance_id"),
		d.QuoteIdentifier("org_id"),
		d.QuoteIdentifier("username"),
		d.QuoteIdentifier("email"),
		d.QuoteIdentifier("display_name"),
		d.QuoteIdentifier("state"),
		d.QuoteIdentifier("password_hash"),
		d.QuoteIdentifier("metadata"),
		d.QuoteIdentifier("created_at"),
		d.QuoteIdentifier("updated_at"),
		d.QuoteIdentifier("instance_id"), d.Placeholder(1),
		d.QuoteIdentifier("id"), d.Placeholder(2),
	)

	r.qGetUserByLoginName = fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s FROM zitadel_users WHERE %s = %s AND (%s = %s OR %s = %s)",
		d.QuoteIdentifier("id"),
		d.QuoteIdentifier("instance_id"),
		d.QuoteIdentifier("org_id"),
		d.QuoteIdentifier("username"),
		d.QuoteIdentifier("email"),
		d.QuoteIdentifier("display_name"),
		d.QuoteIdentifier("state"),
		d.QuoteIdentifier("password_hash"),
		d.QuoteIdentifier("metadata"),
		d.QuoteIdentifier("created_at"),
		d.QuoteIdentifier("updated_at"),
		d.QuoteIdentifier("instance_id"), d.Placeholder(1),
		d.QuoteIdentifier("username"), d.Placeholder(2),
		d.QuoteIdentifier("email"), d.Placeholder(3),
	)

	r.qUpdateUser = fmt.Sprintf(
		"UPDATE zitadel_users SET %s = %s, %s = %s, %s = %s, %s = %s, %s = %s, %s = %s, %s = %s WHERE %s = %s AND %s = %s",
		d.QuoteIdentifier("username"), d.Placeholder(1),
		d.QuoteIdentifier("email"), d.Placeholder(2),
		d.QuoteIdentifier("display_name"), d.Placeholder(3),
		d.QuoteIdentifier("state"), d.Placeholder(4),
		d.QuoteIdentifier("password_hash"), d.Placeholder(5),
		d.QuoteIdentifier("metadata"), d.Placeholder(6),
		d.QuoteIdentifier("updated_at"), d.Placeholder(7),
		d.QuoteIdentifier("instance_id"), d.Placeholder(8),
		d.QuoteIdentifier("id"), d.Placeholder(9),
	)

	r.qDeleteUser = fmt.Sprintf(
		"DELETE FROM zitadel_users WHERE %s = %s AND %s = %s",
		d.QuoteIdentifier("instance_id"), d.Placeholder(1),
		d.QuoteIdentifier("id"), d.Placeholder(2),
	)

	r.qListUsersPrefix = fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s FROM zitadel_users WHERE %s = %s",
		d.QuoteIdentifier("id"),
		d.QuoteIdentifier("instance_id"),
		d.QuoteIdentifier("org_id"),
		d.QuoteIdentifier("username"),
		d.QuoteIdentifier("email"),
		d.QuoteIdentifier("display_name"),
		d.QuoteIdentifier("state"),
		d.QuoteIdentifier("password_hash"),
		d.QuoteIdentifier("metadata"),
		d.QuoteIdentifier("created_at"),
		d.QuoteIdentifier("updated_at"),
		d.QuoteIdentifier("instance_id"), d.Placeholder(1),
	)

	// 2. Orgs Queries
	r.qCreateOrg = fmt.Sprintf(
		"INSERT INTO zitadel_orgs (%s, %s, %s, %s, %s, %s) VALUES (%s, %s, %s, %s, %s, %s)",
		d.QuoteIdentifier("id"),
		d.QuoteIdentifier("instance_id"),
		d.QuoteIdentifier("name"),
		d.QuoteIdentifier("state"),
		d.QuoteIdentifier("created_at"),
		d.QuoteIdentifier("updated_at"),
		d.Placeholder(1),
		d.Placeholder(2),
		d.Placeholder(3),
		d.Placeholder(4),
		d.Placeholder(5),
		d.Placeholder(6),
	)

	r.qGetOrgByID = fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s FROM zitadel_orgs WHERE %s = %s AND %s = %s",
		d.QuoteIdentifier("id"),
		d.QuoteIdentifier("instance_id"),
		d.QuoteIdentifier("name"),
		d.QuoteIdentifier("state"),
		d.QuoteIdentifier("created_at"),
		d.QuoteIdentifier("updated_at"),
		d.QuoteIdentifier("instance_id"), d.Placeholder(1),
		d.QuoteIdentifier("id"), d.Placeholder(2),
	)

	r.qUpdateOrg = fmt.Sprintf(
		"UPDATE zitadel_orgs SET %s = %s, %s = %s, %s = %s WHERE %s = %s AND %s = %s",
		d.QuoteIdentifier("name"), d.Placeholder(1),
		d.QuoteIdentifier("state"), d.Placeholder(2),
		d.QuoteIdentifier("updated_at"), d.Placeholder(3),
		d.QuoteIdentifier("instance_id"), d.Placeholder(4),
		d.QuoteIdentifier("id"), d.Placeholder(5),
	)

	r.qDeleteOrg = fmt.Sprintf(
		"DELETE FROM zitadel_orgs WHERE %s = %s AND %s = %s",
		d.QuoteIdentifier("instance_id"), d.Placeholder(1),
		d.QuoteIdentifier("id"), d.Placeholder(2),
	)

	r.qListOrgsPrefix = fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s FROM zitadel_orgs WHERE %s = %s",
		d.QuoteIdentifier("id"),
		d.QuoteIdentifier("instance_id"),
		d.QuoteIdentifier("name"),
		d.QuoteIdentifier("state"),
		d.QuoteIdentifier("created_at"),
		d.QuoteIdentifier("updated_at"),
		d.QuoteIdentifier("instance_id"), d.Placeholder(1),
	)

	// 3. Projects Queries
	r.qCreateProject = fmt.Sprintf(
		"INSERT INTO zitadel_projects (%s, %s, %s, %s, %s, %s) VALUES (%s, %s, %s, %s, %s, %s)",
		d.QuoteIdentifier("id"),
		d.QuoteIdentifier("instance_id"),
		d.QuoteIdentifier("org_id"),
		d.QuoteIdentifier("name"),
		d.QuoteIdentifier("created_at"),
		d.QuoteIdentifier("updated_at"),
		d.Placeholder(1),
		d.Placeholder(2),
		d.Placeholder(3),
		d.Placeholder(4),
		d.Placeholder(5),
		d.Placeholder(6),
	)

	r.qGetProjectByID = fmt.Sprintf(
		"SELECT %s, %s, %s, %s, %s, %s FROM zitadel_projects WHERE %s = %s AND %s = %s",
		d.QuoteIdentifier("id"),
		d.QuoteIdentifier("instance_id"),
		d.QuoteIdentifier("org_id"),
		d.QuoteIdentifier("name"),
		d.QuoteIdentifier("created_at"),
		d.QuoteIdentifier("updated_at"),
		d.QuoteIdentifier("instance_id"), d.Placeholder(1),
		d.QuoteIdentifier("id"), d.Placeholder(2),
	)

	r.qUpdateProject = fmt.Sprintf(
		"UPDATE zitadel_projects SET %s = %s, %s = %s WHERE %s = %s AND %s = %s",
		d.QuoteIdentifier("name"), d.Placeholder(1),
		d.QuoteIdentifier("updated_at"), d.Placeholder(2),
		d.QuoteIdentifier("instance_id"), d.Placeholder(3),
		d.QuoteIdentifier("id"), d.Placeholder(4),
	)

	r.qDeleteProject = fmt.Sprintf(
		"DELETE FROM zitadel_projects WHERE %s = %s AND %s = %s",
		d.QuoteIdentifier("instance_id"), d.Placeholder(1),
		d.QuoteIdentifier("id"), d.Placeholder(2),
	)
}

// --- UserRepository Implementation ---

func (r *Repository) CreateUser(ctx context.Context, u *storage.User) error {
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	u.UpdatedAt = u.CreatedAt

	var metaJSON []byte
	if len(u.Metadata) > 0 {
		metaJSON, _ = json.Marshal(u.Metadata)
	} else {
		metaJSON = []byte("{}")
	}

	_, err := r.db.ExecContext(ctx, r.qCreateUser,
		u.ID, u.InstanceID, u.OrgID, u.Username, u.Email, u.DisplayName, u.State, u.PasswordHash, string(metaJSON), u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return r.dialect.TranslateError(err)
	}
	return nil
}

func (r *Repository) GetUserByID(ctx context.Context, instanceID, userID string) (*storage.User, error) {
	row := r.db.QueryRowContext(ctx, r.qGetUserByID, instanceID, userID)
	u := &storage.User{}
	var metaStr string
	err := row.Scan(&u.ID, &u.InstanceID, &u.OrgID, &u.Username, &u.Email, &u.DisplayName, &u.State, &u.PasswordHash, &metaStr, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, relational.ErrNotFound
	}
	if err != nil {
		return nil, r.dialect.TranslateError(err)
	}
	if metaStr != "" && metaStr != "{}" {
		_ = json.Unmarshal([]byte(metaStr), &u.Metadata)
	}
	return u, nil
}

func (r *Repository) GetUserByLoginName(ctx context.Context, instanceID, loginName string) (*storage.User, error) {
	row := r.db.QueryRowContext(ctx, r.qGetUserByLoginName, instanceID, loginName, loginName)
	u := &storage.User{}
	var metaStr string
	err := row.Scan(&u.ID, &u.InstanceID, &u.OrgID, &u.Username, &u.Email, &u.DisplayName, &u.State, &u.PasswordHash, &metaStr, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, relational.ErrNotFound
	}
	if err != nil {
		return nil, r.dialect.TranslateError(err)
	}
	if metaStr != "" && metaStr != "{}" {
		_ = json.Unmarshal([]byte(metaStr), &u.Metadata)
	}
	return u, nil
}

func (r *Repository) UpdateUser(ctx context.Context, u *storage.User) error {
	u.UpdatedAt = time.Now()
	var metaJSON []byte
	if len(u.Metadata) > 0 {
		metaJSON, _ = json.Marshal(u.Metadata)
	} else {
		metaJSON = []byte("{}")
	}

	_, err := r.db.ExecContext(ctx, r.qUpdateUser,
		u.Username, u.Email, u.DisplayName, u.State, u.PasswordHash, string(metaJSON), u.UpdatedAt, u.InstanceID, u.ID,
	)
	if err != nil {
		return r.dialect.TranslateError(err)
	}
	return nil
}

func (r *Repository) DeleteUser(ctx context.Context, instanceID, userID string) error {
	_, err := r.db.ExecContext(ctx, r.qDeleteUser, instanceID, userID)
	return r.dialect.TranslateError(err)
}

func (r *Repository) ListUsers(ctx context.Context, filter storage.UserFilter) ([]*storage.User, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	query := r.qListUsersPrefix + " " + r.dialect.PaginationClause(filter.Offset, limit)

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
		if metaStr != "" && metaStr != "{}" {
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

	_, err := r.db.ExecContext(ctx, r.qCreateOrg, o.ID, o.InstanceID, o.Name, o.State, o.CreatedAt, o.UpdatedAt)
	return r.dialect.TranslateError(err)
}

func (r *Repository) GetOrgByID(ctx context.Context, instanceID, orgID string) (*storage.Org, error) {
	row := r.db.QueryRowContext(ctx, r.qGetOrgByID, instanceID, orgID)
	o := &storage.Org{}
	err := row.Scan(&o.ID, &o.InstanceID, &o.Name, &o.State, &o.CreatedAt, &o.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, relational.ErrNotFound
	}
	return o, r.dialect.TranslateError(err)
}

func (r *Repository) UpdateOrg(ctx context.Context, o *storage.Org) error {
	o.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, r.qUpdateOrg, o.Name, o.State, o.UpdatedAt, o.InstanceID, o.ID)
	return r.dialect.TranslateError(err)
}

func (r *Repository) DeleteOrg(ctx context.Context, instanceID, orgID string) error {
	_, err := r.db.ExecContext(ctx, r.qDeleteOrg, instanceID, orgID)
	return r.dialect.TranslateError(err)
}

func (r *Repository) ListOrgs(ctx context.Context, filter storage.OrgFilter) ([]*storage.Org, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	query := r.qListOrgsPrefix + " " + r.dialect.PaginationClause(filter.Offset, limit)
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

	_, err := r.db.ExecContext(ctx, r.qCreateProject, p.ID, p.InstanceID, p.OrgID, p.Name, p.CreatedAt, p.UpdatedAt)
	return r.dialect.TranslateError(err)
}

func (r *Repository) GetProjectByID(ctx context.Context, instanceID, projectID string) (*storage.Project, error) {
	row := r.db.QueryRowContext(ctx, r.qGetProjectByID, instanceID, projectID)
	p := &storage.Project{}
	err := row.Scan(&p.ID, &p.InstanceID, &p.OrgID, &p.Name, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, relational.ErrNotFound
	}
	return p, r.dialect.TranslateError(err)
}

func (r *Repository) UpdateProject(ctx context.Context, p *storage.Project) error {
	p.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx, r.qUpdateProject, p.Name, p.UpdatedAt, p.InstanceID, p.ID)
	return r.dialect.TranslateError(err)
}

func (r *Repository) DeleteProject(ctx context.Context, instanceID, projectID string) error {
	_, err := r.db.ExecContext(ctx, r.qDeleteProject, instanceID, projectID)
	return r.dialect.TranslateError(err)
}
