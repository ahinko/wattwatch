package repository

import (
	"context"
	"database/sql"
	"time"
	"wattwatch/internal/models"

	"github.com/google/uuid"
)

// SpotPriceRepository defines the interface for spot price-related database operations
type SpotPriceRepository interface {
	Repository
	Create(ctx context.Context, spotPrice *models.SpotPrice) error
	CreateBatch(ctx context.Context, spotPrices []models.SpotPrice) error
	Update(ctx context.Context, spotPrice *models.SpotPrice) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.SpotPrice, error)
	List(ctx context.Context, filter SpotPriceFilter) ([]models.SpotPrice, error)
}

// SpotPriceFilter defines the filter options for listing spot prices
type SpotPriceFilter struct {
	ZoneID     *uuid.UUID
	CurrencyID *uuid.UUID
	StartTime  *time.Time
	EndTime    *time.Time
	OrderBy    string
	OrderDesc  bool
	Limit      *int
	Offset     *int
}

type SpotPriceRepositoryImpl struct {
	db *sql.DB
}

func NewSpotPriceRepository(db *sql.DB) *SpotPriceRepositoryImpl {
	return &SpotPriceRepositoryImpl{db: db}
}

func (r *SpotPriceRepositoryImpl) Create(sp *models.SpotPrice) error {
	query := `
		INSERT INTO spot_prices (timestamp, zone_id, currency_id, price)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`

	return r.db.QueryRow(query, sp.Timestamp, sp.ZoneID, sp.CurrencyID, sp.Price).
		Scan(&sp.ID, &sp.CreatedAt, &sp.UpdatedAt)
}

func (r *SpotPriceRepositoryImpl) GetByID(id uuid.UUID) (*models.SpotPrice, error) {
	sp := &models.SpotPrice{}
	query := `
		SELECT id, timestamp, zone_id, currency_id, price, created_at, updated_at
		FROM spot_prices
		WHERE id = $1`

	err := r.db.QueryRow(query, id).Scan(
		&sp.ID, &sp.Timestamp, &sp.ZoneID, &sp.CurrencyID,
		&sp.Price, &sp.CreatedAt, &sp.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return sp, nil
}

func (r *SpotPriceRepositoryImpl) List(zoneID, currencyID *uuid.UUID, startTime, endTime *time.Time) ([]models.SpotPrice, error) {
	query := `
		SELECT id, timestamp, zone_id, currency_id, price, created_at, updated_at
		FROM spot_prices
		WHERE 1=1`
	var args []interface{}
	argCount := 1

	if zoneID != nil {
		query += ` AND zone_id = $` + string(rune('0'+argCount))
		args = append(args, *zoneID)
		argCount++
	}

	if currencyID != nil {
		query += ` AND currency_id = $` + string(rune('0'+argCount))
		args = append(args, *currencyID)
		argCount++
	}

	if startTime != nil {
		query += ` AND timestamp >= $` + string(rune('0'+argCount))
		args = append(args, *startTime)
		argCount++
	}

	if endTime != nil {
		query += ` AND timestamp <= $` + string(rune('0'+argCount))
		args = append(args, *endTime)
		argCount++
	}

	query += ` ORDER BY timestamp DESC, zone_id, currency_id`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var spotPrices []models.SpotPrice
	for rows.Next() {
		var sp models.SpotPrice
		if err := rows.Scan(&sp.ID, &sp.Timestamp, &sp.ZoneID, &sp.CurrencyID,
			&sp.Price, &sp.CreatedAt, &sp.UpdatedAt); err != nil {
			return nil, err
		}
		spotPrices = append(spotPrices, sp)
	}
	return spotPrices, nil
}
