package models

import (
	"time"

	"github.com/google/uuid"
)

// Zone represents a zone in the system
type Zone struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Name      string    `json:"name" db:"name" binding:"required" example:"SE1"`
	Timezone  string    `json:"timezone" db:"timezone" binding:"required" example:"Europe/Stockholm"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CreateZoneRequest represents the request to create a new zone
type CreateZoneRequest struct {
	Name     string `json:"name" binding:"required" example:"SE5"`
	Timezone string `json:"timezone" binding:"required" example:"Europe/Stockholm"`
}

// UpdateZoneRequest represents the request to update a zone
type UpdateZoneRequest struct {
	Name     string `json:"name" binding:"required" example:"SE5"`
	Timezone string `json:"timezone" binding:"required" example:"Europe/Stockholm"`
}
