package postgres

import (
	"context"
	"database/sql"
	"time"
	"wattwatch/internal/repository"

	"github.com/google/uuid"
)

type loginAttemptRepository struct {
	repository.BaseRepository
}

func NewLoginAttemptRepository(db *sql.DB) repository.LoginAttemptRepository {
	return &loginAttemptRepository{
		BaseRepository: repository.NewBaseRepository(db),
	}
}

func (r *loginAttemptRepository) Create(ctx context.Context, userID uuid.UUID, successful bool, ipAddress string, createdAt time.Time) error {
	// First verify the user exists
	var exists bool
	err := r.DB().QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return repository.ErrNotFound
	}

	query := `
		INSERT INTO login_attempts (id, user_id, success, ip, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err = r.DB().ExecContext(ctx, query, uuid.New(), userID, successful, ipAddress, createdAt)
	return err
}

func (r *loginAttemptRepository) GetRecentAttempts(ctx context.Context, userID uuid.UUID, since time.Time) (int, error) {
	// First verify the user exists
	var exists bool
	err := r.DB().QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, repository.ErrNotFound
	}

	var count int
	query := `
		SELECT COUNT(*)
		FROM login_attempts
		WHERE user_id = $1
		AND success = false
		AND created_at >= $2`

	err = r.DB().QueryRowContext(ctx, query, userID, since).Scan(&count)
	return count, err
}

func (r *loginAttemptRepository) ClearAttempts(ctx context.Context, userID uuid.UUID) error {
	// First verify the user exists
	var exists bool
	err := r.DB().QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return repository.ErrNotFound
	}

	query := `DELETE FROM login_attempts WHERE user_id = $1`
	result, err := r.DB().ExecContext(ctx, query, userID)
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
