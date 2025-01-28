package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"

	"github.com/google/uuid"
)

type roleRepository struct {
	repository.BaseRepository
}

// NewRoleRepository creates a new PostgreSQL role repository
func NewRoleRepository(db *sql.DB) repository.RoleRepository {
	return &roleRepository{
		BaseRepository: repository.NewBaseRepository(db),
	}
}

func (r *roleRepository) Create(ctx context.Context, role *models.Role) error {
	// Check if role with same name exists
	var count int
	err := r.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM roles WHERE name = $1 AND deleted_at IS NULL",
		role.Name,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return repository.ErrConflict
	}

	query := `
		INSERT INTO roles (
			id, name, is_protected, is_admin_group,
			created_at, updated_at, deleted_at
		) VALUES (
			$1, $2, $3, $4, $5, $5, NULL
		)
		RETURNING id, created_at, updated_at`

	now := time.Now()
	role.ID = uuid.New()
	role.CreatedAt = now
	role.UpdatedAt = now

	err = r.DB().QueryRowContext(ctx, query,
		role.ID,
		role.Name,
		role.IsProtected,
		role.IsAdminGroup,
		now,
	).Scan(&role.ID, &role.CreatedAt, &role.UpdatedAt)

	if err != nil {
		return err
	}
	return nil
}

func (r *roleRepository) Update(ctx context.Context, role *models.Role) error {
	// Check if role exists and is not protected
	var isProtected bool
	err := r.DB().QueryRowContext(ctx,
		"SELECT is_protected FROM roles WHERE id = $1 AND deleted_at IS NULL",
		role.ID,
	).Scan(&isProtected)
	if err == sql.ErrNoRows {
		return repository.ErrNotFound
	}
	if err != nil {
		return err
	}
	if isProtected {
		return repository.ErrProtectedRole
	}

	// Check if new name conflicts with existing role
	var count int
	err = r.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM roles WHERE name = $1 AND id != $2 AND deleted_at IS NULL",
		role.Name,
		role.ID,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return repository.ErrConflict
	}

	query := `
		UPDATE roles
		SET name = $1,
			is_protected = $2,
			is_admin_group = $3,
			updated_at = $4
		WHERE id = $5 AND deleted_at IS NULL
		RETURNING updated_at`

	result := r.DB().QueryRowContext(ctx, query,
		role.Name,
		role.IsProtected,
		role.IsAdminGroup,
		time.Now(),
		role.ID,
	)

	if err := result.Scan(&role.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *roleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// Check if role is protected
	var isProtected bool
	err := r.DB().QueryRowContext(ctx,
		"SELECT is_protected FROM roles WHERE id = $1 AND deleted_at IS NULL",
		id,
	).Scan(&isProtected)
	if err == sql.ErrNoRows {
		return repository.ErrNotFound
	}
	if err != nil {
		return err
	}
	if isProtected {
		return repository.ErrProtectedRole
	}

	// Check if role is in use
	var inUse bool
	err = r.DB().QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE role_id = $1 AND deleted_at IS NULL)",
		id,
	).Scan(&inUse)
	if err != nil {
		return err
	}
	if inUse {
		return repository.ErrHasAssociatedRecords
	}

	query := `
		UPDATE roles
		SET deleted_at = $1, updated_at = $1
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING deleted_at`

	now := time.Now()
	result := r.DB().QueryRowContext(ctx, query, now, id)

	var deletedAt time.Time
	if err := result.Scan(&deletedAt); err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *roleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Role, error) {
	role := &models.Role{}
	query := `
		SELECT id, name, is_protected, is_admin_group,
			   created_at, updated_at, deleted_at
		FROM roles
		WHERE id = $1 AND deleted_at IS NULL`

	err := r.DB().QueryRowContext(ctx, query, id).Scan(
		&role.ID,
		&role.Name,
		&role.IsProtected,
		&role.IsAdminGroup,
		&role.CreatedAt,
		&role.UpdatedAt,
		&role.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return role, nil
}

func (r *roleRepository) GetByName(ctx context.Context, name string) (*models.Role, error) {
	role := &models.Role{}
	query := `
		SELECT id, name, is_protected, is_admin_group,
			   created_at, updated_at, deleted_at
		FROM roles
		WHERE name = $1 AND deleted_at IS NULL`

	err := r.DB().QueryRowContext(ctx, query, name).Scan(
		&role.ID,
		&role.Name,
		&role.IsProtected,
		&role.IsAdminGroup,
		&role.CreatedAt,
		&role.UpdatedAt,
		&role.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return role, nil
}

func (r *roleRepository) List(ctx context.Context, filter repository.RoleFilter) ([]models.Role, error) {
	conditions := make([]string, 0)
	args := make([]interface{}, 0)
	argCount := 1

	if filter.Search != nil {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argCount))
		args = append(args, "%"+*filter.Search+"%")
		argCount++
	}

	if filter.Protected != nil {
		conditions = append(conditions, fmt.Sprintf("is_protected = $%d", argCount))
		args = append(args, *filter.Protected)
		argCount++
	}

	if filter.AdminGroup != nil {
		conditions = append(conditions, fmt.Sprintf("is_admin_group = $%d", argCount))
		args = append(args, *filter.AdminGroup)
		argCount++
	}

	query := `
		SELECT id, name, is_admin_group, is_protected, created_at, updated_at
		FROM roles
		WHERE deleted_at IS NULL`

	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	// Add ORDER BY clause
	if filter.OrderBy != "" {
		query += fmt.Sprintf(" ORDER BY %s", filter.OrderBy)
		if filter.OrderDesc {
			query += " DESC"
		} else {
			query += " ASC"
		}
	} else {
		query += " ORDER BY name ASC"
	}

	// Add LIMIT and OFFSET
	if filter.Limit != nil {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, *filter.Limit)
		argCount++
	}

	if filter.Offset != nil {
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, *filter.Offset)
	}

	rows, err := r.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var role models.Role
		if err := rows.Scan(
			&role.ID,
			&role.Name,
			&role.IsAdminGroup,
			&role.IsProtected,
			&role.CreatedAt,
			&role.UpdatedAt,
		); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return roles, nil
}
