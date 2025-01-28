package models

import (
	"time"

	"github.com/google/uuid"
)

// SpotPrice represents a spot price in the system
type SpotPrice struct {
	ID         uuid.UUID `json:"id" db:"id"`
	Timestamp  time.Time `json:"timestamp" db:"timestamp" binding:"required"`
	ZoneID     uuid.UUID `json:"zone_id" db:"zone_id" binding:"required"`
	CurrencyID uuid.UUID `json:"currency_id" db:"currency_id" binding:"required"`
	Price      float64   `json:"price" db:"price" binding:"required"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// CreateSpotPriceRequest represents a single spot price in a batch creation request
type CreateSpotPriceRequest struct {
	Timestamp  time.Time `json:"timestamp" binding:"required" example:"2024-03-20T13:00:00Z"`
	ZoneID     uuid.UUID `json:"zone_id" binding:"required"`
	CurrencyID uuid.UUID `json:"currency_id" binding:"required"`
	Price      float64   `json:"price" binding:"required" example:"42.50"`
}

// CreateSpotPricesRequest represents a batch creation request for spot prices
type CreateSpotPricesRequest struct {
	SpotPrices []CreateSpotPriceRequest `json:"spot_prices" binding:"required,min=1"`
}
