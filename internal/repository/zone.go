package repository

import (
	"context"
	"database/sql"
	"wattwatch/internal/models"

	"github.com/google/uuid"
)

// ZoneRepository defines the interface for zone-related database operations
type ZoneRepository interface {
	Repository
	Create(ctx context.Context, zone *models.Zone) error
	Update(ctx context.Context, zone *models.Zone) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Zone, error)
	GetByName(ctx context.Context, name string) (*models.Zone, error)
	List(ctx context.Context, filter ZoneFilter) ([]models.Zone, error)
}

// ZoneFilter defines the filter options for listing zones
type ZoneFilter struct {
	Search    *string // Search by name
	OrderBy   string  // Field to order by
	OrderDesc bool    // Order descending
	Limit     *int    // Limit results
	Offset    *int    // Offset results
}

type ZoneRepositoryImpl struct {
	db *sql.DB
}

func NewZoneRepository(db *sql.DB) *ZoneRepositoryImpl {
	return &ZoneRepositoryImpl{db: db}
}

func (r *ZoneRepositoryImpl) Create(zone *models.Zone) error {
	query := `
		INSERT INTO zones (name, timezone)
		VALUES ($1, $2)
		RETURNING id, created_at, updated_at`

	return r.db.QueryRow(query, zone.Name, zone.Timezone).
		Scan(&zone.ID, &zone.CreatedAt, &zone.UpdatedAt)
}

func (r *ZoneRepositoryImpl) GetByID(id uuid.UUID) (*models.Zone, error) {
	zone := &models.Zone{}
	query := `
		SELECT id, name, timezone, created_at, updated_at
		FROM zones
		WHERE id = $1`

	err := r.db.QueryRow(query, id).Scan(
		&zone.ID, &zone.Name, &zone.Timezone,
		&zone.CreatedAt, &zone.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrZoneNotFound
	}
	if err != nil {
		return nil, err
	}
	return zone, nil
}

func (r *ZoneRepositoryImpl) GetByName(name string) (*models.Zone, error) {
	zone := &models.Zone{}
	query := `
		SELECT id, name, timezone, created_at, updated_at
		FROM zones
		WHERE name = $1`

	err := r.db.QueryRow(query, name).Scan(
		&zone.ID, &zone.Name, &zone.Timezone,
		&zone.CreatedAt, &zone.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrZoneNotFound
	}
	if err != nil {
		return nil, err
	}
	return zone, nil
}

func (r *ZoneRepositoryImpl) List() ([]models.Zone, error) {
	query := `
		SELECT id, name, timezone, created_at, updated_at
		FROM zones
		ORDER BY name`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var zones []models.Zone
	for rows.Next() {
		var zone models.Zone
		if err := rows.Scan(&zone.ID, &zone.Name, &zone.Timezone,
			&zone.CreatedAt, &zone.UpdatedAt); err != nil {
			return nil, err
		}
		zones = append(zones, zone)
	}
	return zones, nil
}
