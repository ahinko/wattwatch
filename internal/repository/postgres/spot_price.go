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

type spotPriceRepository struct {
	repository.BaseRepository
}

// NewSpotPriceRepository creates a new PostgreSQL spot price repository
func NewSpotPriceRepository(db *sql.DB) repository.SpotPriceRepository {
	return &spotPriceRepository{
		BaseRepository: repository.NewBaseRepository(db),
	}
}

func (r *spotPriceRepository) Create(ctx context.Context, spotPrice *models.SpotPrice) error {
	query := `
		INSERT INTO spot_prices (id, timestamp, zone_id, currency_id, price, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $6)
		RETURNING id, created_at, updated_at`

	now := time.Now()
	spotPrice.ID = uuid.New()

	err := r.DB().QueryRowContext(ctx, query,
		spotPrice.ID,
		spotPrice.Timestamp,
		spotPrice.ZoneID,
		spotPrice.CurrencyID,
		spotPrice.Price,
		now,
	).Scan(&spotPrice.ID, &spotPrice.CreatedAt, &spotPrice.UpdatedAt)

	if err != nil {
		return err
	}
	return nil
}

func (r *spotPriceRepository) CreateBatch(ctx context.Context, spotPrices []models.SpotPrice) error {
	if len(spotPrices) == 0 {
		return nil
	}

	// Build the query for batch upsert
	valueStrings := make([]string, 0, len(spotPrices))
	valueArgs := make([]interface{}, 0, len(spotPrices)*7)
	now := time.Now()

	for i, sp := range spotPrices {
		if sp.ID == uuid.Nil {
			sp.ID = uuid.New()
		}
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			i*7+1, i*7+2, i*7+3, i*7+4, i*7+5, i*7+6, i*7+7))
		valueArgs = append(valueArgs,
			sp.ID,
			sp.Timestamp,
			sp.ZoneID,
			sp.CurrencyID,
			sp.Price,
			now,
			now,
		)
	}

	query := fmt.Sprintf(`
		INSERT INTO spot_prices (id, timestamp, zone_id, currency_id, price, created_at, updated_at)
		VALUES %s
		ON CONFLICT (timestamp, zone_id, currency_id) DO UPDATE
		SET price = EXCLUDED.price,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at, updated_at`, strings.Join(valueStrings, ","))

	rows, err := r.DB().QueryContext(ctx, query, valueArgs...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Update the spot prices with the returned values
	i := 0
	for rows.Next() {
		if err := rows.Scan(&spotPrices[i].ID, &spotPrices[i].CreatedAt, &spotPrices[i].UpdatedAt); err != nil {
			return err
		}
		i++
	}

	return rows.Err()
}

func (r *spotPriceRepository) Update(ctx context.Context, spotPrice *models.SpotPrice) error {
	query := `
		UPDATE spot_prices
		SET timestamp = $1, zone_id = $2, currency_id = $3, price = $4, updated_at = $5
		WHERE id = $6
		RETURNING updated_at`

	result := r.DB().QueryRowContext(ctx, query,
		spotPrice.Timestamp,
		spotPrice.ZoneID,
		spotPrice.CurrencyID,
		spotPrice.Price,
		time.Now(),
		spotPrice.ID,
	)

	if err := result.Scan(&spotPrice.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrNotFound
		}
		return err
	}
	return nil
}

func (r *spotPriceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM spot_prices WHERE id = $1`
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

func (r *spotPriceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.SpotPrice, error) {
	query := `
		SELECT id, timestamp, zone_id, currency_id, price, created_at, updated_at
		FROM spot_prices
		WHERE id = $1`

	spotPrice := &models.SpotPrice{}
	err := r.DB().QueryRowContext(ctx, query, id).Scan(
		&spotPrice.ID,
		&spotPrice.Timestamp,
		&spotPrice.ZoneID,
		&spotPrice.CurrencyID,
		&spotPrice.Price,
		&spotPrice.CreatedAt,
		&spotPrice.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return spotPrice, nil
}

func (r *spotPriceRepository) List(ctx context.Context, filter repository.SpotPriceFilter) ([]models.SpotPrice, error) {
	conditions := make([]string, 0)
	args := make([]interface{}, 0)
	argCount := 1

	if filter.ZoneID != nil {
		conditions = append(conditions, fmt.Sprintf("zone_id = $%d", argCount))
		args = append(args, *filter.ZoneID)
		argCount++
	}

	if filter.CurrencyID != nil {
		conditions = append(conditions, fmt.Sprintf("currency_id = $%d", argCount))
		args = append(args, *filter.CurrencyID)
		argCount++
	}

	if filter.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp >= $%d", argCount))
		args = append(args, *filter.StartTime)
		argCount++
	}

	if filter.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("timestamp <= $%d", argCount))
		args = append(args, *filter.EndTime)
		argCount++
	}

	query := `
		SELECT id, timestamp, zone_id, currency_id, price, created_at, updated_at
		FROM spot_prices`

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
		query += " ORDER BY timestamp DESC"
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

	var spotPrices []models.SpotPrice
	for rows.Next() {
		var sp models.SpotPrice
		if err := rows.Scan(
			&sp.ID,
			&sp.Timestamp,
			&sp.ZoneID,
			&sp.CurrencyID,
			&sp.Price,
			&sp.CreatedAt,
			&sp.UpdatedAt,
		); err != nil {
			return nil, err
		}
		spotPrices = append(spotPrices, sp)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return spotPrices, nil
}
