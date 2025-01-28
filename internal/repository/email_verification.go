package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

const (
	TokenExpirationHours    = 24
	VerificationTokenLength = 32
)

type EmailVerification struct {
	ID         uuid.UUID  `db:"id"`
	UserID     uuid.UUID  `db:"user_id"`
	Token      string     `db:"token"`
	ExpiresAt  time.Time  `db:"expires_at"`
	VerifiedAt *time.Time `db:"verified_at"`
	CreatedAt  time.Time  `db:"created_at"`
}

type EmailVerificationRepository interface {
	Create(ctx context.Context, userID uuid.UUID) (*EmailVerification, error)
	Verify(ctx context.Context, token string) error
}

func NewEmailVerificationRepository(db *sql.DB) EmailVerificationRepository {
	return &emailVerificationRepositoryImpl{db: db}
}

type emailVerificationRepositoryImpl struct {
	db *sql.DB
}

func generateToken() (string, error) {
	bytes := make([]byte, VerificationTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (r *emailVerificationRepositoryImpl) Create(ctx context.Context, userID uuid.UUID) (*EmailVerification, error) {
	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	verification := &EmailVerification{
		ID:        uuid.New(),
		UserID:    userID,
		Token:     token,
		ExpiresAt: time.Now().Add(TokenExpirationHours * time.Hour),
	}

	query := `
		INSERT INTO email_verifications (id, user_id, token, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at`

	err = r.db.QueryRowContext(ctx, query,
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

func (r *emailVerificationRepositoryImpl) Verify(ctx context.Context, token string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get verification record
	var verification EmailVerification
	err = tx.QueryRowContext(ctx, `
		SELECT id, user_id, token, expires_at, verified_at
		FROM email_verifications
		WHERE token = $1
		AND verified_at IS NULL`,
		token,
	).Scan(
		&verification.ID,
		&verification.UserID,
		&verification.Token,
		&verification.ExpiresAt,
		&verification.VerifiedAt,
	)

	if err == sql.ErrNoRows {
		return ErrTokenInvalid
	}
	if err != nil {
		return err
	}

	// Check if token has expired
	if time.Now().After(verification.ExpiresAt) {
		return ErrTokenExpired
	}

	// Mark as verified
	_, err = tx.ExecContext(ctx, `
		UPDATE email_verifications
		SET verified_at = CURRENT_TIMESTAMP
		WHERE id = $1`,
		verification.ID,
	)
	if err != nil {
		return err
	}

	// Update user's email_verified status
	_, err = tx.ExecContext(ctx, `
		UPDATE users
		SET email_verified = true,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $1`,
		verification.UserID,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}
