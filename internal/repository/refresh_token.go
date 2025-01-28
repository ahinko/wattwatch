package repository

import (
	"context"
	"time"
	"wattwatch/internal/models"

	"github.com/google/uuid"
)

// RefreshTokenRepository defines the interface for refresh token operations
type RefreshTokenRepository interface {
	Repository
	Create(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error
	GetByToken(ctx context.Context, token string) (*models.RefreshToken, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.RefreshToken, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByToken(ctx context.Context, token string) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
	IsValid(ctx context.Context, token string) (bool, error)
}

// RefreshTokenFilter defines the filter options for listing refresh tokens
type RefreshTokenFilter struct {
	UserID        *uuid.UUID // Filter by user ID
	ExpiresAfter  *time.Time // Filter by expiration time
	ExpiresBefore *time.Time // Filter by expiration time
	CreatedAfter  *time.Time // Filter by creation time
	CreatedBefore *time.Time // Filter by creation time
	OrderBy       string     // Field to order by
	OrderDesc     bool       // Order descending
	Limit         *int       // Limit results
	Offset        *int       // Offset results
}
