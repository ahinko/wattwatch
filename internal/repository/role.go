package repository

import (
	"context"
	"database/sql"
	"wattwatch/internal/models"

	"github.com/google/uuid"
)

// RoleRepository defines the interface for role-related database operations
type RoleRepository interface {
	Repository
	Create(ctx context.Context, role *models.Role) error
	Update(ctx context.Context, role *models.Role) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Role, error)
	GetByName(ctx context.Context, name string) (*models.Role, error)
	List(ctx context.Context, filter RoleFilter) ([]models.Role, error)
}

// RoleFilter defines the filter options for listing roles
type RoleFilter struct {
	Search     *string // Search by name
	Protected  *bool   // Filter by protected status
	AdminGroup *bool   // Filter by admin group status
	OrderBy    string  // Field to order by
	OrderDesc  bool    // Order descending
	Limit      *int    // Limit results
	Offset     *int    // Offset results
}

type RoleRepositoryImpl struct {
	db *sql.DB
}

func NewRoleRepository(db *sql.DB) *RoleRepositoryImpl {
	return &RoleRepositoryImpl{db: db}
}

func (r *RoleRepositoryImpl) GetByID(id uuid.UUID) (*models.Role, error) {
	role := &models.Role{}
	query := `
		SELECT id, name, is_admin_group, is_protected, created_at, updated_at
		FROM roles
		WHERE id = $1 AND deleted_at IS NULL`

	err := r.db.QueryRow(query, id).Scan(
		&role.ID,
		&role.Name,
		&role.IsAdminGroup,
		&role.IsProtected,
		&role.CreatedAt,
		&role.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrRoleNotFound
	}
	if err != nil {
		return nil, err
	}
	return role, nil
}

func (r *RoleRepositoryImpl) GetByName(name string) (*models.Role, error) {
	role := &models.Role{}
	query := `
		SELECT id, name, is_protected, is_admin_group, created_at, updated_at
		FROM roles
		WHERE name = $1 AND deleted_at IS NULL`

	err := r.db.QueryRow(query, name).Scan(
		&role.ID,
		&role.Name,
		&role.IsProtected,
		&role.IsAdminGroup,
		&role.CreatedAt,
		&role.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrRoleNotFound
	}
	if err != nil {
		return nil, err
	}
	return role, nil
}

func (r *RoleRepositoryImpl) List() ([]models.Role, error) {
	query := `
		SELECT id, name, is_admin_group, is_protected, created_at, updated_at
		FROM roles
		WHERE deleted_at IS NULL
		ORDER BY name`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var role models.Role
		err := rows.Scan(
			&role.ID,
			&role.Name,
			&role.IsAdminGroup,
			&role.IsProtected,
			&role.CreatedAt,
			&role.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	return roles, nil
}

func (r *RoleRepositoryImpl) Create(role *models.Role) error {
	// Check if role with this name already exists
	existingRole, err := r.GetByName(role.Name)
	if err != nil && err != ErrRoleNotFound {
		return err
	}
	if existingRole != nil {
		return ErrRoleExists
	}

	query := `
		INSERT INTO roles (name, is_admin_group, is_protected)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`

	return r.db.QueryRow(
		query,
		role.Name,
		role.IsAdminGroup,
		role.IsProtected,
	).Scan(&role.ID, &role.CreatedAt, &role.UpdatedAt)
}

func (r *RoleRepositoryImpl) Update(role *models.Role) error {
	// Check if role exists
	existingRole, err := r.GetByID(role.ID)
	if err == ErrRoleNotFound {
		return err
	}
	if err != nil {
		return err
	}

	// Check if role with this name already exists
	if role.Name != existingRole.Name {
		otherRole, err := r.GetByName(role.Name)
		if err != nil && err != ErrRoleNotFound {
			return err
		}
		if otherRole != nil {
			return ErrRoleExists
		}
	}

	query := `
		UPDATE roles
		SET name = $1, is_admin_group = $2, is_protected = $3, updated_at = CURRENT_TIMESTAMP
		WHERE id = $4 AND deleted_at IS NULL
		RETURNING created_at, updated_at`

	err = r.db.QueryRow(
		query,
		role.Name,
		role.IsAdminGroup,
		role.IsProtected,
		role.ID,
	).Scan(&role.CreatedAt, &role.UpdatedAt)

	if err == sql.ErrNoRows {
		return ErrRoleNotFound
	}
	if err != nil {
		return err
	}

	return nil
}

func (r *RoleRepositoryImpl) Delete(id uuid.UUID) error {
	// Check if role is in use
	var inUse bool
	checkQuery := `SELECT EXISTS(SELECT 1 FROM users WHERE role_id = $1 AND deleted_at IS NULL)`
	if err := r.db.QueryRow(checkQuery, id).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return ErrRoleInUse
	}

	query := `
		UPDATE roles
		SET deleted_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND NOT is_protected AND deleted_at IS NULL`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrRoleNotFound
	}

	return nil
}
