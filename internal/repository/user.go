package repository

import (
	"context"
	"database/sql"
	"time"
	"wattwatch/internal/models"

	"github.com/google/uuid"
)

// UserRepository defines the interface for user-related database operations
type UserRepository interface {
	Repository
	Create(ctx context.Context, user *models.User) error
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	List(ctx context.Context, filter UserFilter) ([]models.User, error)
	UpdatePassword(ctx context.Context, id uuid.UUID, hashedPassword string) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID, lastLoginAt time.Time) error
	UpdateFailedAttempts(ctx context.Context, id uuid.UUID, attempts int) error
	VerifyEmail(ctx context.Context, id uuid.UUID) error
	IncrementFailedAttempts(ctx context.Context, username string) error
	ResetFailedAttempts(ctx context.Context, username string) error
}

// UserFilter defines the filter options for listing users
type UserFilter struct {
	Search    *string // Search by username or email
	RoleID    *uuid.UUID
	OrderBy   string // Field to order by
	OrderDesc bool   // Order descending
	Limit     *int   // Limit results
	Offset    *int   // Offset results
}

type userRepositoryImpl struct {
	db              *sql.DB
	passwordHistory PasswordHistoryRepository
}

func NewUserRepository(db *sql.DB, passwordHistory PasswordHistoryRepository) *userRepositoryImpl {
	return &userRepositoryImpl{
		db:              db,
		passwordHistory: passwordHistory,
	}
}

func (r *userRepositoryImpl) Create(user *models.User) error {
	query := `
		INSERT INTO users (username, password, email, role_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`

	return r.db.QueryRow(
		query,
		user.Username,
		user.Password,
		user.Email,
		user.RoleID,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
}

func (r *userRepositoryImpl) GetByUsername(username string) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, username, password, email, role_id, email_verified, 
		       created_at, updated_at, last_login_at, failed_login_attempts,
		       last_failed_login, password_changed_at, deleted_at
		FROM users
		WHERE username = $1 AND deleted_at IS NULL`

	err := r.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Email,
		&user.RoleID,
		&user.EmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
		&user.FailedLoginAttempts,
		&user.LastFailedLogin,
		&user.PasswordChangedAt,
		&user.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *userRepositoryImpl) GetByID(id uuid.UUID) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, username, password, email, role_id, email_verified,
		       created_at, updated_at, last_login_at, failed_login_attempts,
		       last_failed_login, password_changed_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL`

	err := r.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Email,
		&user.RoleID,
		&user.EmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
		&user.FailedLoginAttempts,
		&user.LastFailedLogin,
		&user.PasswordChangedAt,
		&user.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *userRepositoryImpl) Update(user *models.User) error {
	// Check if email already exists for a different user
	var count int
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM users WHERE email = $1 AND id != $2 AND deleted_at IS NULL",
		user.Email,
		user.ID,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrEmailExists
	}

	query := `
		UPDATE users
		SET email = $1, role_id = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $3 AND deleted_at IS NULL
		RETURNING updated_at`

	return r.db.QueryRow(
		query,
		user.Email,
		user.RoleID,
		user.ID,
	).Scan(&user.UpdatedAt)
}

func (r *userRepositoryImpl) Delete(id uuid.UUID) error {
	// Check if user is an admin by joining with roles table
	var isAdmin bool
	err := r.db.QueryRow(`
		SELECT r.is_admin_group 
		FROM users u 
		JOIN roles r ON u.role_id = r.id 
		WHERE u.id = $1 AND u.deleted_at IS NULL`,
		id,
	).Scan(&isAdmin)

	if err == sql.ErrNoRows {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}

	if isAdmin {
		return ErrAdminDelete
	}

	query := `
		UPDATE users
		SET deleted_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *userRepositoryImpl) UpdatePassword(ctx context.Context, userID uuid.UUID, hashedPassword string) error {
	// Check if password was recently used
	if err := r.passwordHistory.CheckReuse(ctx, userID, hashedPassword); err != nil {
		return err
	}

	// Start transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		UPDATE users
		SET password = $1, password_changed_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND deleted_at IS NULL`

	result, err := tx.ExecContext(ctx, query, hashedPassword, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}

	// Add password to history
	if err := r.passwordHistory.Add(ctx, userID, hashedPassword); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *userRepositoryImpl) List() ([]models.User, error) {
	query := `
		SELECT id, username, password, email, role_id, email_verified,
		       created_at, updated_at, last_login_at, failed_login_attempts,
		       last_failed_login, password_changed_at, deleted_at
		FROM users
		WHERE deleted_at IS NULL
		ORDER BY username`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Password,
			&user.Email,
			&user.RoleID,
			&user.EmailVerified,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.LastLoginAt,
			&user.FailedLoginAttempts,
			&user.LastFailedLogin,
			&user.PasswordChangedAt,
			&user.DeletedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

func (r *userRepositoryImpl) IncrementFailedAttempts(username string) error {
	query := `
		UPDATE users
		SET failed_login_attempts = failed_login_attempts + 1,
		    last_failed_login = CURRENT_TIMESTAMP
		WHERE username = $1 AND deleted_at IS NULL`

	_, err := r.db.Exec(query, username)
	return err
}

func (r *userRepositoryImpl) ResetFailedAttempts(username string) error {
	query := `
		UPDATE users
		SET failed_login_attempts = 0,
		    last_failed_login = NULL
		WHERE username = $1 AND deleted_at IS NULL`

	_, err := r.db.Exec(query, username)
	return err
}

func (r *userRepositoryImpl) UpdateLastLogin(userID uuid.UUID) error {
	query := `
		UPDATE users
		SET last_login_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND deleted_at IS NULL`

	_, err := r.db.Exec(query, userID)
	return err
}

func (r *userRepositoryImpl) GetByEmail(email string) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, username, password, email, role_id, email_verified,
		       created_at, updated_at, last_login_at, failed_login_attempts,
		       last_failed_login, password_changed_at, deleted_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL`

	err := r.db.QueryRow(query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Email,
		&user.RoleID,
		&user.EmailVerified,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.LastLoginAt,
		&user.FailedLoginAttempts,
		&user.LastFailedLogin,
		&user.PasswordChangedAt,
		&user.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *userRepositoryImpl) MarkEmailVerified(userID uuid.UUID) error {
	query := `
		UPDATE users
		SET email_verified = true
		WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.db.Exec(query, userID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}
