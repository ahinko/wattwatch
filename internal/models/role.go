package models

import (
	"time"

	"github.com/google/uuid"
)

// Role represents a role in the system
type Role struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	Name         string     `json:"name" db:"name" binding:"required,min=3,max=50,nospaces"`
	IsProtected  bool       `json:"is_protected" db:"is_protected"`
	IsAdminGroup bool       `json:"is_admin_group" db:"is_admin_group"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// CreateRoleRequest represents the request to create a new role
type CreateRoleRequest struct {
	Name         string `json:"name" binding:"required,min=3,max=50,nospaces"`
	IsProtected  bool   `json:"is_protected"`
	IsAdminGroup bool   `json:"is_admin_group"`
}

// UpdateRoleRequest represents the request to update a role
type UpdateRoleRequest struct {
	Name         string `json:"name" binding:"required,min=3,max=50,nospaces"`
	IsProtected  bool   `json:"is_protected"`
	IsAdminGroup bool   `json:"is_admin_group"`
}
