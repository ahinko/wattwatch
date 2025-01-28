package postgres

import (
	"context"
	"database/sql"
	"time"
	"wattwatch/internal/repository"

	"github.com/google/uuid"
)

type passwordResetRepository struct {
	repository.BaseRepository
}

func NewPasswordResetRepository(db *sql.DB) repository.PasswordResetRepository {
	return &passwordResetRepository{
		BaseRepository: repository.NewBaseRepository(db),
	}
}

func (r *passwordResetRepository) Create(ctx context.Context, userID uuid.UUID) (*repository.PasswordReset, error) {
	// First verify the user exists
	var exists bool
	err := r.DB().QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, repository.ErrNotFound
	}

	reset := &repository.PasswordReset{
		ID:        uuid.New(),
		UserID:    userID,
		Token:     uuid.New().String(),
		ExpiresAt: time.Now().Add(repository.ResetTokenExpiration),
	}

	query := `
		INSERT INTO password_resets (id, user_id, token, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at`

	err = r.DB().QueryRowContext(ctx, query, reset.ID, reset.UserID, reset.Token, reset.ExpiresAt).
		Scan(&reset.CreatedAt)
	if err != nil {
		return nil, err
	}

	return reset, nil
}

func (r *passwordResetRepository) GetByToken(ctx context.Context, token string) (*repository.PasswordReset, error) {
	reset := &repository.PasswordReset{}
	query := `
		SELECT id, user_id, token, expires_at, used_at, created_at
		FROM password_resets
		WHERE token = $1`

	err := r.DB().QueryRowContext(ctx, query, token).Scan(
		&reset.ID,
		&reset.UserID,
		&reset.Token,
		&reset.ExpiresAt,
		&reset.UsedAt,
		&reset.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, repository.ErrResetTokenInvalid
	}
	if err != nil {
		return nil, err
	}

	if reset.UsedAt != nil {
		return nil, repository.ErrResetTokenUsed
	}

	if time.Now().After(reset.ExpiresAt) {
		return nil, repository.ErrResetTokenExpired
	}

	return reset, nil
}

func (r *passwordResetRepository) MarkAsUsed(ctx context.Context, id uuid.UUID) error {
	// First check if token exists and is not already used
	var usedAt *time.Time
	err := r.DB().QueryRowContext(ctx,
		"SELECT used_at FROM password_resets WHERE id = $1",
		id).Scan(&usedAt)

	if err == sql.ErrNoRows {
		return repository.ErrResetTokenInvalid
	}
	if err != nil {
		return err
	}
	if usedAt != nil {
		return repository.ErrResetTokenInvalid
	}

	query := `
		UPDATE password_resets
		SET used_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND used_at IS NULL`

	result, err := r.DB().ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return repository.ErrResetTokenInvalid
	}

	return nil
}
