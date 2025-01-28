package models

import (
	"time"

	"github.com/google/uuid"
)

// Currency represents a currency in the system
type Currency struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Name      string    `json:"name" db:"name" binding:"required,len=3" example:"USD"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CreateCurrencyRequest represents the request to create a new currency
type CreateCurrencyRequest struct {
	Name string `json:"name" binding:"required,len=3" message:"Currency code must be exactly 3 letters (e.g. USD, EUR)" example:"USD"`
}

// UpdateCurrencyRequest represents the request to update a currency
type UpdateCurrencyRequest struct {
	Name string `json:"name" binding:"required,len=3" message:"Currency code must be exactly 3 letters (e.g. USD, EUR)" example:"USD"`
}
