package postgres

import (
	"context"
	"database/sql"
	"time"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type passwordHistoryRepository struct {
	repository.BaseRepository
}

// NewPasswordHistoryRepository creates a new PostgreSQL password history repository
func NewPasswordHistoryRepository(db *sql.DB) repository.PasswordHistoryRepository {
	return &passwordHistoryRepository{
		BaseRepository: repository.NewBaseRepository(db),
	}
}

func (r *passwordHistoryRepository) Add(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := `
		INSERT INTO password_history (
			id, user_id, password_hash, created_at
		) VALUES (
			$1, $2, $3, $4
		)`

	id := uuid.New()
	now := time.Now()

	_, err := r.DB().ExecContext(ctx, query,
		id,
		userID,
		passwordHash,
		now,
	)

	return err
}

func (r *passwordHistoryRepository) CheckReuse(ctx context.Context, userID uuid.UUID, newPasswordHash string) error {
	query := `
		SELECT password_hash 
		FROM password_history
		WHERE user_id = $1
		AND created_at > NOW() - INTERVAL '90 days'
		ORDER BY created_at DESC
		LIMIT 5`

	rows, err := r.DB().QueryContext(ctx, query, userID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var oldHash string
		if err := rows.Scan(&oldHash); err != nil {
			return err
		}

		// Compare the new password with the old hash
		if err := bcrypt.CompareHashAndPassword([]byte(oldHash), []byte(newPasswordHash)); err == nil {
			// If there's no error, it means the passwords match
			return repository.ErrPasswordReuse
		}
	}

	return rows.Err()
}

func (r *passwordHistoryRepository) CleanupOld(ctx context.Context, olderThan time.Duration) error {
	query := `
		DELETE FROM password_history 
		WHERE created_at < $1`

	cutoff := time.Now().Add(-olderThan)
	_, err := r.DB().ExecContext(ctx, query, cutoff)
	return err
}

func (r *passwordHistoryRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.PasswordHistory, error) {
	query := `
		SELECT id, user_id, password_hash, created_at
		FROM password_history
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := r.DB().QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []models.PasswordHistory
	for rows.Next() {
		var history models.PasswordHistory
		if err := rows.Scan(
			&history.ID,
			&history.UserID,
			&history.PasswordHash,
			&history.CreatedAt,
		); err != nil {
			return nil, err
		}
		histories = append(histories, history)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return histories, nil
}
