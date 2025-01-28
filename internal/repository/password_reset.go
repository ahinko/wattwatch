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
	ResetTokenLength     = 32
	ResetTokenExpiration = 1 * time.Hour
)

type PasswordReset struct {
	ID        uuid.UUID  `db:"id"`
	UserID    uuid.UUID  `db:"user_id"`
	Token     string     `db:"token"`
	ExpiresAt time.Time  `db:"expires_at"`
	UsedAt    *time.Time `db:"used_at"`
	CreatedAt time.Time  `db:"created_at"`
}

type PasswordResetRepository interface {
	Create(ctx context.Context, userID uuid.UUID) (*PasswordReset, error)
	GetByToken(ctx context.Context, token string) (*PasswordReset, error)
	MarkAsUsed(ctx context.Context, id uuid.UUID) error
}

type passwordResetRepositoryImpl struct {
	db *sql.DB
}

func NewPasswordResetRepository(db *sql.DB) PasswordResetRepository {
	return &passwordResetRepositoryImpl{db: db}
}

func generateResetToken() (string, error) {
	bytes := make([]byte, ResetTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (r *passwordResetRepositoryImpl) Create(ctx context.Context, userID uuid.UUID) (*PasswordReset, error) {
	token, err := generateResetToken()
	if err != nil {
		return nil, err
	}

	reset := &PasswordReset{
		ID:        uuid.New(),
		UserID:    userID,
		Token:     token,
		ExpiresAt: time.Now().Add(ResetTokenExpiration),
	}

	query := `
		INSERT INTO password_resets (id, user_id, token, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at`

	err = r.db.QueryRowContext(ctx,
		query,
		reset.ID,
		reset.UserID,
		reset.Token,
		reset.ExpiresAt,
	).Scan(&reset.CreatedAt)

	if err != nil {
		return nil, err
	}

	return reset, nil
}

func (r *passwordResetRepositoryImpl) GetByToken(ctx context.Context, token string) (*PasswordReset, error) {
	reset := &PasswordReset{}
	query := `
		SELECT id, user_id, token, expires_at, used_at, created_at
		FROM password_resets
		WHERE token = $1`

	err := r.db.QueryRowContext(ctx, query, token).Scan(
		&reset.ID,
		&reset.UserID,
		&reset.Token,
		&reset.ExpiresAt,
		&reset.UsedAt,
		&reset.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrResetTokenInvalid
	}
	if err != nil {
		return nil, err
	}

	if reset.UsedAt != nil {
		return nil, ErrResetTokenUsed
	}

	if time.Now().After(reset.ExpiresAt) {
		return nil, ErrResetTokenExpired
	}

	return reset, nil
}

func (r *passwordResetRepositoryImpl) MarkAsUsed(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE password_resets
		SET used_at = CURRENT_TIMESTAMP
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrResetTokenInvalid
	}

	return nil
}
