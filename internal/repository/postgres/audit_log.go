package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type auditLogRepository struct {
	repository.BaseRepository
}

// NewAuditLogRepository creates a new PostgreSQL audit log repository
func NewAuditLogRepository(db *sql.DB) repository.AuditLogRepository {
	return &auditLogRepository{
		BaseRepository: repository.NewBaseRepository(db),
	}
}

func (r *auditLogRepository) Create(ctx context.Context, log *models.CreateAuditLogRequest) error {
	query := `
		INSERT INTO audit_logs (
			id, user_id, action, entity_type, entity_id,
			description, metadata, ip_address, user_agent,
			created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)`

	id := uuid.New()
	now := time.Now()

	_, err := r.DB().ExecContext(ctx, query,
		id,
		log.UserID,
		log.Action,
		log.EntityType,
		log.EntityID,
		log.Description,
		log.Metadata,
		log.IPAddress,
		log.UserAgent,
		now,
	)

	return err
}

func (r *auditLogRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	query := `
		SELECT id, user_id, action, entity_type, entity_id,
			   description, metadata, ip_address, user_agent,
			   created_at
		FROM audit_logs
		WHERE id = $1`

	var log models.AuditLog
	err := r.DB().QueryRowContext(ctx, query, id).Scan(
		&log.ID,
		&log.UserID,
		&log.Action,
		&log.EntityType,
		&log.EntityID,
		&log.Description,
		&log.Metadata,
		&log.IPAddress,
		&log.UserAgent,
		&log.CreatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	return &log, nil
}

func (r *auditLogRepository) buildListQuery(filter repository.AuditLogFilter) (string, []interface{}) {
	var conditions []string
	var params []interface{}
	paramCount := 1

	query := `
		SELECT id, user_id, action, entity_type, entity_id,
			   description, metadata, ip_address, user_agent,
			   created_at
		FROM audit_logs`

	if filter.UserID != nil {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", paramCount))
		params = append(params, filter.UserID)
		paramCount++
	}

	if len(filter.Actions) > 0 {
		conditions = append(conditions, fmt.Sprintf("action = ANY($%d)", paramCount))
		params = append(params, pq.Array(filter.Actions))
		paramCount++
	}

	if len(filter.EntityTypes) > 0 {
		conditions = append(conditions, fmt.Sprintf("entity_type = ANY($%d)", paramCount))
		params = append(params, pq.Array(filter.EntityTypes))
		paramCount++
	}

	if len(filter.EntityIDs) > 0 {
		conditions = append(conditions, fmt.Sprintf("entity_id = ANY($%d)", paramCount))
		params = append(params, pq.Array(filter.EntityIDs))
		paramCount++
	}

	if filter.IPAddress != nil {
		conditions = append(conditions, fmt.Sprintf("ip_address = $%d", paramCount))
		params = append(params, filter.IPAddress)
		paramCount++
	}

	if filter.CreatedBefore != nil {
		conditions = append(conditions, fmt.Sprintf("created_at < $%d", paramCount))
		params = append(params, filter.CreatedBefore)
		paramCount++
	}

	if filter.CreatedAfter != nil {
		conditions = append(conditions, fmt.Sprintf("created_at > $%d", paramCount))
		params = append(params, filter.CreatedAfter)
		paramCount++
	}

	if filter.SearchTerm != nil {
		conditions = append(conditions, fmt.Sprintf("(description ILIKE $%d OR metadata ILIKE $%d)", paramCount, paramCount))
		searchPattern := "%" + *filter.SearchTerm + "%"
		params = append(params, searchPattern)
		paramCount++
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	if filter.OrderBy != "" {
		query += fmt.Sprintf(" ORDER BY %s", filter.OrderBy)
		if filter.OrderDesc {
			query += " DESC"
		}
	} else {
		query += " ORDER BY created_at DESC"
	}

	if filter.Limit != nil {
		query += fmt.Sprintf(" LIMIT $%d", paramCount)
		params = append(params, filter.Limit)
		paramCount++
	}

	if filter.Offset != nil {
		query += fmt.Sprintf(" OFFSET $%d", paramCount)
		params = append(params, filter.Offset)
	}

	return query, params
}

func (r *auditLogRepository) List(ctx context.Context, filter repository.AuditLogFilter) ([]models.AuditLog, error) {
	query, params := r.buildListQuery(filter)
	return r.queryLogs(ctx, query, params...)
}

func (r *auditLogRepository) GetByUserID(ctx context.Context, userID uuid.UUID, filter repository.AuditLogFilter) ([]models.AuditLog, error) {
	filter.UserID = &userID
	return r.List(ctx, filter)
}

func (r *auditLogRepository) GetByEntityTypeAndID(ctx context.Context, entityType, entityID string, filter repository.AuditLogFilter) ([]models.AuditLog, error) {
	filter.EntityTypes = []string{entityType}
	filter.EntityIDs = []string{entityID}
	return r.List(ctx, filter)
}

func (r *auditLogRepository) CleanupOld(ctx context.Context, olderThan time.Duration) error {
	query := `DELETE FROM audit_logs WHERE created_at < $1`
	cutoff := time.Now().Add(-olderThan)
	_, err := r.DB().ExecContext(ctx, query, cutoff)
	return err
}

func (r *auditLogRepository) queryLogs(ctx context.Context, query string, args ...interface{}) ([]models.AuditLog, error) {
	rows, err := r.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.Action,
			&log.EntityType,
			&log.EntityID,
			&log.Description,
			&log.Metadata,
			&log.IPAddress,
			&log.UserAgent,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return logs, nil
}
