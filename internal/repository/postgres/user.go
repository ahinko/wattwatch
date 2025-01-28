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

type userRepository struct {
	repository.BaseRepository
}

// NewUserRepository creates a new PostgreSQL user repository
func NewUserRepository(db *sql.DB) repository.UserRepository {
	return &userRepository{
		BaseRepository: repository.NewBaseRepository(db),
	}
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (
			id, username, password, email, email_verified, role_id,
			last_login_at, last_failed_login, password_changed_at,
			failed_login_attempts, deleted_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12
		)
		RETURNING id, created_at, updated_at`

	now := time.Now()
	user.ID = uuid.New()
	user.CreatedAt = now
	user.UpdatedAt = now

	err := r.DB().QueryRowContext(ctx, query,
		user.ID,
		user.Username,
		user.Password,
		user.Email,
		user.EmailVerified,
		user.RoleID,
		user.LastLoginAt,
		user.LastFailedLogin,
		user.PasswordChangedAt,
		user.FailedLoginAttempts,
		user.DeletedAt,
		now,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return err
	}
	return nil
}

func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	// Check if new email conflicts with existing user
	var count int
	err := r.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE email = $1 AND id != $2 AND deleted_at IS NULL",
		user.Email,
		user.ID,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return repository.ErrConflict
	}

	query := `
		UPDATE users
		SET username = $1,
			email = $2,
			email_verified = $3,
			role_id = $4,
			updated_at = $5
		WHERE id = $6 AND deleted_at IS NULL
		RETURNING updated_at`

	result := r.DB().QueryRowContext(ctx, query,
		user.Username,
		user.Email,
		user.EmailVerified,
		user.RoleID,
		time.Now(),
		user.ID,
	)

	if err := result.Scan(&user.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *userRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// First check if user is an admin
	var isAdmin bool
	err := r.DB().QueryRowContext(ctx, `
		SELECT r.is_admin_group 
		FROM users u 
		JOIN roles r ON u.role_id = r.id 
		WHERE u.id = $1 AND u.deleted_at IS NULL`,
		id,
	).Scan(&isAdmin)
	if err == sql.ErrNoRows {
		return repository.ErrUserNotFound
	}
	if err != nil {
		return err
	}
	if isAdmin {
		return repository.ErrAdminDelete
	}

	query := `
		UPDATE users
		SET deleted_at = $1, updated_at = $1
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING deleted_at`

	now := time.Now()
	result := r.DB().QueryRowContext(ctx, query, now, id)

	var deletedAt time.Time
	if err := result.Scan(&deletedAt); err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrUserNotFound
		}
		return err
	}
	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `
		SELECT 
			u.id, u.username, u.password, u.email, u.email_verified,
			u.role_id, u.last_login_at, u.last_failed_login,
			u.password_changed_at, u.failed_login_attempts,
			u.deleted_at, u.created_at, u.updated_at,
			r.id, r.name, r.is_admin_group, r.is_protected,
			r.created_at, r.updated_at
		FROM users u
		LEFT JOIN roles r ON u.role_id = r.id
		WHERE u.id = $1 AND u.deleted_at IS NULL`

	user := &models.User{Role: &models.Role{}}
	err := r.DB().QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Email,
		&user.EmailVerified,
		&user.RoleID,
		&user.LastLoginAt,
		&user.LastFailedLogin,
		&user.PasswordChangedAt,
		&user.FailedLoginAttempts,
		&user.DeletedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.Role.ID,
		&user.Role.Name,
		&user.Role.IsAdminGroup,
		&user.Role.IsProtected,
		&user.Role.CreatedAt,
		&user.Role.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, repository.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	query := `
		SELECT 
			u.id, u.username, u.password, u.email, u.email_verified,
			u.role_id, u.last_login_at, u.last_failed_login,
			u.password_changed_at, u.failed_login_attempts,
			u.deleted_at, u.created_at, u.updated_at,
			r.id, r.name, r.is_admin_group, r.is_protected,
			r.created_at, r.updated_at
		FROM users u
		LEFT JOIN roles r ON u.role_id = r.id
		WHERE u.username = $1 AND u.deleted_at IS NULL`

	user := &models.User{Role: &models.Role{}}
	err := r.DB().QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Email,
		&user.EmailVerified,
		&user.RoleID,
		&user.LastLoginAt,
		&user.LastFailedLogin,
		&user.PasswordChangedAt,
		&user.FailedLoginAttempts,
		&user.DeletedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.Role.ID,
		&user.Role.Name,
		&user.Role.IsAdminGroup,
		&user.Role.IsProtected,
		&user.Role.CreatedAt,
		&user.Role.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, repository.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT 
			u.id, u.username, u.password, u.email, u.email_verified,
			u.role_id, u.last_login_at, u.last_failed_login,
			u.password_changed_at, u.failed_login_attempts,
			u.deleted_at, u.created_at, u.updated_at,
			r.id, r.name, r.is_admin_group, r.is_protected,
			r.created_at, r.updated_at
		FROM users u
		LEFT JOIN roles r ON u.role_id = r.id
		WHERE u.email = $1 AND u.deleted_at IS NULL`

	user := &models.User{Role: &models.Role{}}
	err := r.DB().QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Email,
		&user.EmailVerified,
		&user.RoleID,
		&user.LastLoginAt,
		&user.LastFailedLogin,
		&user.PasswordChangedAt,
		&user.FailedLoginAttempts,
		&user.DeletedAt,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.Role.ID,
		&user.Role.Name,
		&user.Role.IsAdminGroup,
		&user.Role.IsProtected,
		&user.Role.CreatedAt,
		&user.Role.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, repository.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *userRepository) List(ctx context.Context, filter repository.UserFilter) ([]models.User, error) {
	conditions := make([]string, 0)
	args := make([]interface{}, 0)
	argCount := 1

	if filter.Search != nil {
		conditions = append(conditions, fmt.Sprintf("(username ILIKE $%d OR email ILIKE $%d)", argCount, argCount))
		args = append(args, "%"+*filter.Search+"%")
		argCount++
	}

	if filter.RoleID != nil {
		conditions = append(conditions, fmt.Sprintf("role_id = $%d", argCount))
		args = append(args, *filter.RoleID)
		argCount++
	}

	query := `
		SELECT u.id, u.username, u.email, u.role_id, u.email_verified,
		       u.created_at, u.updated_at, u.last_login_at, u.failed_login_attempts,
		       u.last_failed_login, u.password_changed_at,
		       r.name as role_name, r.is_admin_group, r.is_protected
		FROM users u
		JOIN roles r ON u.role_id = r.id
		WHERE u.deleted_at IS NULL`

	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	// Add ORDER BY clause
	if filter.OrderBy != "" {
		query += fmt.Sprintf(" ORDER BY u.%s", filter.OrderBy)
		if filter.OrderDesc {
			query += " DESC"
		} else {
			query += " ASC"
		}
	} else {
		query += " ORDER BY u.username ASC"
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

	var users []models.User
	for rows.Next() {
		var user models.User
		user.Role = &models.Role{}

		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.RoleID,
			&user.EmailVerified,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.LastLoginAt,
			&user.FailedLoginAttempts,
			&user.LastFailedLogin,
			&user.PasswordChangedAt,
			&user.Role.Name,
			&user.Role.IsAdminGroup,
			&user.Role.IsProtected,
		)
		if err != nil {
			return nil, err
		}

		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (r *userRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID, lastLogin time.Time) error {
	query := `
		UPDATE users
		SET last_login_at = $1, updated_at = $2
		WHERE id = $3 AND deleted_at IS NULL
		RETURNING last_login_at`

	now := time.Now()
	result := r.DB().QueryRowContext(ctx, query, lastLogin, now, id)

	var updatedLastLogin time.Time
	if err := result.Scan(&updatedLastLogin); err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *userRepository) UpdateFailedLogin(ctx context.Context, id uuid.UUID, lastFailedLogin time.Time, failedAttempts int) error {
	query := `
		UPDATE users
		SET last_failed_login = $1, failed_login_attempts = $2, updated_at = $3
		WHERE id = $4 AND deleted_at IS NULL
		RETURNING last_failed_login, failed_login_attempts`

	now := time.Now()
	result := r.DB().QueryRowContext(ctx, query, lastFailedLogin, failedAttempts, now, id)

	var updatedLastFailedLogin time.Time
	var updatedFailedAttempts int
	if err := result.Scan(&updatedLastFailedLogin, &updatedFailedAttempts); err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *userRepository) UpdatePassword(ctx context.Context, id uuid.UUID, hashedPassword string) error {
	query := `
		UPDATE users
		SET password = $1, password_changed_at = $2, updated_at = $2
		WHERE id = $3 AND deleted_at IS NULL
		RETURNING password_changed_at`

	now := time.Now()
	result := r.DB().QueryRowContext(ctx, query, hashedPassword, now, id)

	var passwordChangedAt time.Time
	if err := result.Scan(&passwordChangedAt); err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *userRepository) VerifyEmail(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE users
		SET email_verified = true, password_changed_at = $1, updated_at = $1
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING email_verified, password_changed_at`

	now := time.Now()
	result := r.DB().QueryRowContext(ctx, query, now, id)

	var emailVerified bool
	var passwordChangedAt time.Time
	if err := result.Scan(&emailVerified, &passwordChangedAt); err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *userRepository) IncrementFailedAttempts(ctx context.Context, username string) error {
	query := `
		UPDATE users 
		SET failed_login_attempts = failed_login_attempts + 1,
		    last_failed_login = CURRENT_TIMESTAMP
		WHERE username = $1 AND deleted_at IS NULL`

	result, err := r.DB().ExecContext(ctx, query, username)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return repository.ErrNotFound
	}

	return nil
}

func (r *userRepository) ResetFailedAttempts(ctx context.Context, username string) error {
	query := `
		UPDATE users 
		SET failed_login_attempts = 0,
		    last_failed_login = NULL
		WHERE username = $1 AND deleted_at IS NULL`

	result, err := r.DB().ExecContext(ctx, query, username)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return repository.ErrNotFound
	}

	return nil
}

func (r *userRepository) UpdateFailedAttempts(ctx context.Context, id uuid.UUID, attempts int) error {
	query := `
		UPDATE users
		SET failed_login_attempts = $1,
		    last_failed_login = CASE WHEN $1 > 0 THEN CURRENT_TIMESTAMP ELSE NULL END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND deleted_at IS NULL`

	result, err := r.DB().ExecContext(ctx, query, attempts, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return repository.ErrNotFound
	}

	return nil
}
