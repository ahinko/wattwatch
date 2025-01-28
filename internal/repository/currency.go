package repository

import (
	"context"
	"wattwatch/internal/models"

	"github.com/google/uuid"
)

// CurrencyRepository defines the interface for currency-related database operations
type CurrencyRepository interface {
	Repository
	Create(ctx context.Context, currency *models.Currency) error
	Update(ctx context.Context, currency *models.Currency) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Currency, error)
	GetByName(ctx context.Context, name string) (*models.Currency, error)
	List(ctx context.Context) ([]models.Currency, error)
}
