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
)

type zoneRepository struct {
	repository.BaseRepository
}

// NewZoneRepository creates a new PostgreSQL zone repository
func NewZoneRepository(db *sql.DB) repository.ZoneRepository {
	return &zoneRepository{
		BaseRepository: repository.NewBaseRepository(db),
	}
}

func (r *zoneRepository) Create(ctx context.Context, zone *models.Zone) error {
	// Validate timezone
	if _, err := time.LoadLocation(zone.Timezone); err != nil {
		return repository.ErrInvalidTimezone
	}

	// Check if zone with same name exists
	var count int
	err := r.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM zones WHERE name = $1",
		zone.Name,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return repository.ErrConflict
	}

	query := `
		INSERT INTO zones (id, name, timezone, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		RETURNING id, created_at, updated_at`

	now := time.Now()
	zone.ID = uuid.New()

	err = r.DB().QueryRowContext(ctx, query,
		zone.ID,
		zone.Name,
		zone.Timezone,
		now,
	).Scan(&zone.ID, &zone.CreatedAt, &zone.UpdatedAt)

	if err != nil {
		if strings.Contains(err.Error(), "zones_name_key") {
			return repository.ErrConflict
		}
		return err
	}
	return nil
}

func (r *zoneRepository) Update(ctx context.Context, zone *models.Zone) error {
	// Validate timezone
	if _, err := time.LoadLocation(zone.Timezone); err != nil {
		return repository.ErrInvalidTimezone
	}

	// Check if zone exists
	var exists bool
	err := r.DB().QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM zones WHERE id = $1)",
		zone.ID,
	).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return repository.ErrNotFound
	}

	// Check if new name conflicts with existing zone
	var count int
	err = r.DB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM zones WHERE name = $1 AND id != $2",
		zone.Name,
		zone.ID,
	).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return repository.ErrConflict
	}

	query := `
		UPDATE zones
		SET name = $1, timezone = $2, updated_at = $3
		WHERE id = $4
		RETURNING updated_at`

	result := r.DB().QueryRowContext(ctx, query,
		zone.Name,
		zone.Timezone,
		time.Now(),
		zone.ID,
	)

	if err := result.Scan(&zone.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrNotFound
		}
		if strings.Contains(err.Error(), "zones_name_key") {
			return repository.ErrConflict
		}
		return err
	}
	return nil
}

func (r *zoneRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// First check if there are any spot prices using this zone
	var count int
	err := r.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM spot_prices WHERE zone_id = $1
	`, id).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return repository.ErrHasAssociatedRecords
	}

	query := `DELETE FROM zones WHERE id = $1`
	result, err := r.DB().ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *zoneRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Zone, error) {
	query := `
		SELECT id, name, timezone, created_at, updated_at
		FROM zones
		WHERE id = $1`

	zone := &models.Zone{}
	err := r.DB().QueryRowContext(ctx, query, id).Scan(
		&zone.ID,
		&zone.Name,
		&zone.Timezone,
		&zone.CreatedAt,
		&zone.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return zone, nil
}

func (r *zoneRepository) GetByName(ctx context.Context, name string) (*models.Zone, error) {
	query := `
		SELECT id, name, timezone, created_at, updated_at
		FROM zones
		WHERE name = $1`

	zone := &models.Zone{}
	err := r.DB().QueryRowContext(ctx, query, name).Scan(
		&zone.ID,
		&zone.Name,
		&zone.Timezone,
		&zone.CreatedAt,
		&zone.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return zone, nil
}

func (r *zoneRepository) List(ctx context.Context, filter repository.ZoneFilter) ([]models.Zone, error) {
	conditions := make([]string, 0)
	args := make([]interface{}, 0)
	argCount := 1

	if filter.Search != nil {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argCount))
		args = append(args, "%"+*filter.Search+"%")
		argCount++
	}

	query := `
		SELECT id, name, timezone, created_at, updated_at
		FROM zones`

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Add ORDER BY clause
	if filter.OrderBy != "" {
		query += fmt.Sprintf(" ORDER BY %s", filter.OrderBy)
		if filter.OrderDesc {
			query += " DESC"
		} else {
			query += " ASC"
		}
	} else {
		query += " ORDER BY name ASC"
	}

	// Add LIMIT and OFFSET
	if filter.Limit != nil {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, *filter.Limit)
		argCount++
	}

	if filter.Offset != nil {
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, *filter.Offset)
	}

	rows, err := r.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var zones []models.Zone
	for rows.Next() {
		var zone models.Zone
		if err := rows.Scan(
			&zone.ID,
			&zone.Name,
			&zone.Timezone,
			&zone.CreatedAt,
			&zone.UpdatedAt,
		); err != nil {
			return nil, err
		}
		zones = append(zones, zone)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return zones, nil
}
