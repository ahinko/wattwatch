package postgres

import (
	"context"
	"database/sql"
	"time"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"

	"github.com/google/uuid"
)

type refreshTokenRepository struct {
	repository.BaseRepository
}

// NewRefreshTokenRepository creates a new PostgreSQL refresh token repository
func NewRefreshTokenRepository(db *sql.DB) repository.RefreshTokenRepository {
	return &refreshTokenRepository{
		BaseRepository: repository.NewBaseRepository(db),
	}
}

func (r *refreshTokenRepository) Create(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error {
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
		INSERT INTO refresh_tokens (
			id, user_id, token, expires_at, created_at
		) VALUES (
			$1, $2, $3, $4, $5
		)`

	id := uuid.New()
	now := time.Now()

	_, err = r.DB().ExecContext(ctx, query,
		id,
		userID,
		token,
		expiresAt,
		now,
	)

	return err
}

func (r *refreshTokenRepository) GetByToken(ctx context.Context, token string) (*models.RefreshToken, error) {
	refreshToken := &models.RefreshToken{}
	query := `
		SELECT id, user_id, token, expires_at, created_at
		FROM refresh_tokens
		WHERE token = $1`

	err := r.DB().QueryRowContext(ctx, query, token).Scan(
		&refreshToken.ID,
		&refreshToken.UserID,
		&refreshToken.Token,
		&refreshToken.ExpiresAt,
		&refreshToken.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, repository.ErrTokenInvalid
	}
	if err != nil {
		return nil, err
	}

	if time.Now().After(refreshToken.ExpiresAt) {
		return nil, repository.ErrTokenExpired
	}

	return refreshToken, nil
}

func (r *refreshTokenRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.RefreshToken, error) {
	// First verify the user exists
	var exists bool
	err := r.DB().QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, repository.ErrNotFound
	}

	query := `
		SELECT id, user_id, token, expires_at, created_at
		FROM refresh_tokens
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := r.DB().QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []models.RefreshToken
	for rows.Next() {
		var rt models.RefreshToken
		err := rows.Scan(
			&rt.ID,
			&rt.UserID,
			&rt.Token,
			&rt.ExpiresAt,
			&rt.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, rt)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return tokens, nil
}

func (r *refreshTokenRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM refresh_tokens WHERE id = $1`
	result, err := r.DB().ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return repository.ErrTokenInvalid
	}

	return nil
}

func (r *refreshTokenRepository) DeleteByToken(ctx context.Context, token string) error {
	query := `DELETE FROM refresh_tokens WHERE token = $1`
	result, err := r.DB().ExecContext(ctx, query, token)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return repository.ErrTokenInvalid
	}

	return nil
}

func (r *refreshTokenRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM refresh_tokens WHERE user_id = $1`
	_, err := r.DB().ExecContext(ctx, query, userID)
	return err
}

func (r *refreshTokenRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM refresh_tokens WHERE expires_at < $1`
	_, err := r.DB().ExecContext(ctx, query, time.Now())
	return err
}

func (r *refreshTokenRepository) IsValid(ctx context.Context, token string) (bool, error) {
	query := `
		SELECT expires_at
		FROM refresh_tokens
		WHERE token = $1`

	var expiresAt time.Time
	err := r.DB().QueryRowContext(ctx, query, token).Scan(&expiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, repository.ErrTokenInvalid
		}
		return false, err
	}

	return time.Now().Before(expiresAt), nil
}
