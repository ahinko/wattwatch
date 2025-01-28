package repository

import (
	"database/sql"
	"wattwatch/internal/models"

	"github.com/google/uuid"
)

type AuditRepository struct {
	db *sql.DB
}

func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Create(log *models.AuditLog) error {
	query := `
		INSERT INTO audit_logs (user_id, action, description, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	return r.db.QueryRow(
		query,
		log.UserID,
		log.Action,
		log.Description,
		log.IPAddress,
		log.UserAgent,
	).Scan(&log.ID, &log.CreatedAt)
}

func (r *AuditRepository) GetByUsername(username string) ([]models.AuditLog, error) {
	query := `
		SELECT a.id, a.user_id, a.action, a.description, a.ip_address, a.user_agent, a.created_at
		FROM audit_logs a
		JOIN users u ON a.user_id = u.id
		WHERE u.username = $1
		ORDER BY a.created_at DESC`

	rows, err := r.db.Query(query, username)
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
			&log.Description,
			&log.IPAddress,
			&log.UserAgent,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, nil
}

func (r *AuditRepository) GetByUserID(userID uuid.UUID) ([]models.AuditLog, error) {
	query := `
		SELECT id, user_id, action, description, ip_address, user_agent, created_at
		FROM audit_logs
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Query(query, userID)
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
			&log.Description,
			&log.IPAddress,
			&log.UserAgent,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, nil
}
