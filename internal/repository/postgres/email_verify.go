package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
	"wattwatch/internal/repository"

	"github.com/google/uuid"
)

type emailVerificationRepository struct {
	db *sql.DB
}

func NewEmailVerificationRepository(db *sql.DB) repository.EmailVerificationRepository {
	return &emailVerificationRepository{db: db}
}

func generateToken() (string, error) {
	bytes := make([]byte, repository.VerificationTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (r *emailVerificationRepository) Create(ctx context.Context, userID uuid.UUID) (*repository.EmailVerification, error) {
	// First verify the user exists
	var exists bool
	err := r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, repository.ErrNotFound
	}

	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	verification := &repository.EmailVerification{
		ID:        uuid.New(),
		UserID:    userID,
		Token:     token,
		ExpiresAt: time.Now().Add(repository.TokenExpirationHours * time.Hour),
	}

	query := `
		INSERT INTO email_verifications (id, user_id, token, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at`

	err = r.db.QueryRowContext(
		ctx,
		query,
		verification.ID,
		verification.UserID,
		verification.Token,
		verification.ExpiresAt,
	).Scan(&verification.CreatedAt)

	if err != nil {
		return nil, err
	}

	return verification, nil
}

func (r *emailVerificationRepository) Verify(ctx context.Context, token string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var userID uuid.UUID
	var expiresAt time.Time
	query := `
		UPDATE email_verifications
		SET verified_at = CURRENT_TIMESTAMP
		WHERE token = $1 AND verified_at IS NULL
		RETURNING user_id, expires_at`

	err = tx.QueryRowContext(ctx, query, token).Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return repository.ErrTokenInvalid
	}
	if err != nil {
		return err
	}

	if time.Now().After(expiresAt) {
		return repository.ErrTokenExpired
	}

	// Update user's email_verified status
	query = `UPDATE users SET email_verified = true WHERE id = $1`
	if _, err := tx.ExecContext(ctx, query, userID); err != nil {
		return err
	}

	return tx.Commit()
}
