package repository

import (
	"context"
	"time"
	"wattwatch/internal/models"

	"github.com/google/uuid"
)

// AuditLogRepository defines the interface for audit log operations
type AuditLogRepository interface {
	Repository
	Create(ctx context.Context, log *models.CreateAuditLogRequest) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error)
	List(ctx context.Context, filter AuditLogFilter) ([]models.AuditLog, error)
	GetByUserID(ctx context.Context, userID uuid.UUID, filter AuditLogFilter) ([]models.AuditLog, error)
	GetByEntityTypeAndID(ctx context.Context, entityType, entityID string, filter AuditLogFilter) ([]models.AuditLog, error)
	CleanupOld(ctx context.Context, olderThan time.Duration) error
}

// AuditLogFilter defines the filter options for listing audit logs
type AuditLogFilter struct {
	UserID        *uuid.UUID           // Filter by user ID
	Actions       []models.AuditAction // Filter by actions
	EntityTypes   []string             // Filter by entity types
	EntityIDs     []string             // Filter by entity IDs
	IPAddress     *string              // Filter by IP address
	CreatedBefore *time.Time           // Filter by creation time
	CreatedAfter  *time.Time           // Filter by creation time
	SearchTerm    *string              // Search in description and metadata
	OrderBy       string               // Field to order by
	OrderDesc     bool                 // Order descending
	Limit         *int                 // Limit results
	Offset        *int                 // Offset results
}
