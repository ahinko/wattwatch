package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"wattwatch/internal/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrPasswordReuse = errors.New("password was recently used")
)

// PasswordHistoryRepository defines the interface for password history operations
type PasswordHistoryRepository interface {
	Repository
	Add(ctx context.Context, userID uuid.UUID, passwordHash string) error
	CheckReuse(ctx context.Context, userID uuid.UUID, newPasswordHash string) error
	CleanupOld(ctx context.Context, olderThan time.Duration) error
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.PasswordHistory, error)
}

// PasswordHistoryFilter defines the filter options for listing password history
type PasswordHistoryFilter struct {
	UserID        *uuid.UUID // Filter by user ID
	CreatedBefore *time.Time // Filter by creation time
	CreatedAfter  *time.Time // Filter by creation time
	OrderBy       string     // Field to order by
	OrderDesc     bool       // Order descending
	Limit         *int       // Limit results
	Offset        *int       // Offset results
}

type PasswordHistoryRepositoryImpl struct {
	db *sql.DB
}

func NewPasswordHistoryRepository(db *sql.DB) *PasswordHistoryRepositoryImpl {
	return &PasswordHistoryRepositoryImpl{db: db}
}

func (r *PasswordHistoryRepositoryImpl) Add(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := `
		INSERT INTO password_history (user_id, password_hash)
		VALUES ($1, $2)`

	_, err := r.db.ExecContext(ctx, query, userID, passwordHash)
	return err
}

func (r *PasswordHistoryRepositoryImpl) CheckReuse(ctx context.Context, userID uuid.UUID, newPassword string) error {
	query := `
		SELECT password_hash FROM password_history
		WHERE user_id = $1
		AND created_at > NOW() - INTERVAL '90 days'`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return err
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(newPassword)); err == nil {
			// If CompareHashAndPassword returns nil, it means the password matches
			return ErrPasswordReuse
		}
	}

	return rows.Err()
}

func (r *PasswordHistoryRepositoryImpl) CleanupOld(ctx context.Context, olderThan time.Duration) error {
	query := `DELETE FROM password_history WHERE created_at < NOW() - INTERVAL $1`
	_, err := r.db.ExecContext(ctx, query, olderThan)
	return err
}

func (r *PasswordHistoryRepositoryImpl) GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.PasswordHistory, error) {
	query := `
		SELECT id, user_id, password_hash, created_at
		FROM password_history
		WHERE user_id = $1`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var histories []models.PasswordHistory
	for rows.Next() {
		var history models.PasswordHistory
		if err := rows.Scan(&history.ID, &history.UserID, &history.PasswordHash, &history.CreatedAt); err != nil {
			return nil, err
		}
		histories = append(histories, history)
	}

	return histories, rows.Err()
}
