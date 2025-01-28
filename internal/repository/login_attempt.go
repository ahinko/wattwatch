package repository

import (
	"context"
	"database/sql"
	"time"
	"wattwatch/internal/models"

	"github.com/google/uuid"
)

const (
	MaxLoginAttempts = 5
	LockoutDuration  = 15 * time.Minute
)

type LoginAttemptRepository interface {
	Create(ctx context.Context, userID uuid.UUID, successful bool, ipAddress string, createdAt time.Time) error
	GetRecentAttempts(ctx context.Context, userID uuid.UUID, since time.Time) (int, error)
	ClearAttempts(ctx context.Context, userID uuid.UUID) error
}

type LoginAttemptRepositoryImpl struct {
	db *sql.DB
}

func NewLoginAttemptRepository(db *sql.DB) *LoginAttemptRepositoryImpl {
	return &LoginAttemptRepositoryImpl{db: db}
}

func (r *LoginAttemptRepositoryImpl) RecordAttempt(username, ipAddress string) error {
	// Get user ID from username
	var userID uuid.UUID
	err := r.db.QueryRow("SELECT id FROM users WHERE username = $1", username).Scan(&userID)
	if err == sql.ErrNoRows {
		// User doesn't exist, but we still want to record the attempt
		query := `
			INSERT INTO login_attempts (id, user_id, success, ip, created_at)
			VALUES ($1, NULL, false, $2, CURRENT_TIMESTAMP)`
		_, err = r.db.Exec(query, uuid.New(), ipAddress)
		return err
	} else if err != nil {
		return err
	}

	query := `
		INSERT INTO login_attempts (id, user_id, success, ip, created_at)
		VALUES ($1, $2, false, $3, CURRENT_TIMESTAMP)`

	_, err = r.db.Exec(query, uuid.New(), userID, ipAddress)
	return err
}

func (r *LoginAttemptRepositoryImpl) GetRecentAttempts(username string) (int, error) {
	// Get user ID from username
	var userID uuid.UUID
	err := r.db.QueryRow("SELECT id FROM users WHERE username = $1", username).Scan(&userID)
	if err == sql.ErrNoRows {
		// User doesn't exist, return 0 attempts
		return 0, nil
	} else if err != nil {
		return 0, err
	}

	query := `
		SELECT COUNT(*)
		FROM login_attempts
		WHERE user_id = $1
		AND success = false
		AND created_at > $2`

	cutoff := time.Now().Add(-LockoutDuration)
	var count int
	err = r.db.QueryRow(query, userID, cutoff).Scan(&count)
	return count, err
}

func (r *LoginAttemptRepositoryImpl) ClearAttempts(username string) error {
	// Get user ID from username
	var userID uuid.UUID
	err := r.db.QueryRow("SELECT id FROM users WHERE username = $1", username).Scan(&userID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	query := `DELETE FROM login_attempts WHERE user_id = $1`
	_, err = r.db.Exec(query, userID)
	return err
}

func (r *LoginAttemptRepositoryImpl) Create(attempt *models.LoginAttempt) error {
	if attempt.ID == uuid.Nil {
		attempt.ID = uuid.New()
	}

	query := `
		INSERT INTO login_attempts (id, user_id, success, ip, created_at)
		VALUES ($1, $2, $3, $4, $5)`

	_, err := r.db.Exec(
		query,
		attempt.ID,
		attempt.UserID,
		attempt.Success,
		attempt.IP,
		attempt.CreatedAt,
	)
	return err
}
