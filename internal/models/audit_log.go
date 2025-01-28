package models

import (
	"time"

	"github.com/google/uuid"
)

// AuditAction represents the type of action performed
type AuditAction string

const (
	AuditActionCreate AuditAction = "create"
	AuditActionUpdate AuditAction = "update"
	AuditActionDelete AuditAction = "delete"
	AuditActionRead   AuditAction = "read"
	AuditActionLogin  AuditAction = "login"
	AuditActionLogout AuditAction = "logout"
)

// AuditLog represents a record of system activity
type AuditLog struct {
	ID          uuid.UUID   `json:"id" db:"id"`
	UserID      *uuid.UUID  `json:"user_id" db:"user_id"`         // Optional: action might be system-generated
	Action      AuditAction `json:"action" db:"action"`           // The type of action performed
	EntityType  string      `json:"entity_type" db:"entity_type"` // The type of entity affected (e.g., "user", "zone", "spot_price")
	EntityID    string      `json:"entity_id" db:"entity_id"`     // The ID of the affected entity
	Description string      `json:"description" db:"description"` // Human-readable description of the action
	Metadata    string      `json:"metadata" db:"metadata"`       // JSON string containing additional context
	IPAddress   string      `json:"ip_address" db:"ip_address"`   // IP address of the requester
	UserAgent   string      `json:"user_agent" db:"user_agent"`   // User agent of the requester
	CreatedAt   time.Time   `json:"created_at" db:"created_at"`
}

// CreateAuditLogRequest represents the request to create a new audit log entry
type CreateAuditLogRequest struct {
	UserID      *uuid.UUID  `json:"user_id"`
	Action      AuditAction `json:"action" binding:"required"`
	EntityType  string      `json:"entity_type" binding:"required"`
	EntityID    string      `json:"entity_id" binding:"required"`
	Description string      `json:"description" binding:"required"`
	Metadata    string      `json:"metadata"`
	IPAddress   string      `json:"ip_address"`
	UserAgent   string      `json:"user_agent"`
}
