package postgres

import (
	"context"
	"database/sql"
	"time"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type currencyRepository struct {
	repository.BaseRepository
}

// NewCurrencyRepository creates a new PostgreSQL currency repository
func NewCurrencyRepository(db *sql.DB) repository.CurrencyRepository {
	return &currencyRepository{
		BaseRepository: repository.NewBaseRepository(db),
	}
}

func (r *currencyRepository) Create(ctx context.Context, currency *models.Currency) error {
	query := `
		INSERT INTO currencies (id, name, created_at, updated_at)
		VALUES ($1, $2, $3, $3)
		RETURNING id, created_at, updated_at`

	now := time.Now()
	currency.ID = uuid.New()

	err := r.DB().QueryRowContext(ctx, query,
		currency.ID,
		currency.Name,
		now,
	).Scan(&currency.ID, &currency.CreatedAt, &currency.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return repository.ErrConflict
		}
		return err
	}
	return nil
}

func (r *currencyRepository) Update(ctx context.Context, currency *models.Currency) error {
	query := `
		UPDATE currencies
		SET name = $1, updated_at = $2
		WHERE id = $3
		RETURNING updated_at`

	result := r.DB().QueryRowContext(ctx, query,
		currency.Name,
		time.Now(),
		currency.ID,
	)

	if err := result.Scan(&currency.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return repository.ErrNotFound
		}
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "unique_violation" {
			return repository.ErrConflict
		}
		return err
	}
	return nil
}

func (r *currencyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// First check if there are any spot prices using this currency
	var count int
	err := r.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM spot_prices WHERE currency_id = $1
	`, id).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return repository.ErrHasAssociatedRecords
	}

	query := `DELETE FROM currencies WHERE id = $1`
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

func (r *currencyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Currency, error) {
	query := `
		SELECT id, name, created_at, updated_at
		FROM currencies
		WHERE id = $1`

	currency := &models.Currency{}
	err := r.DB().QueryRowContext(ctx, query, id).Scan(
		&currency.ID,
		&currency.Name,
		&currency.CreatedAt,
		&currency.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return currency, nil
}

func (r *currencyRepository) GetByName(ctx context.Context, name string) (*models.Currency, error) {
	query := `
		SELECT id, name, created_at, updated_at
		FROM currencies
		WHERE name = $1`

	currency := &models.Currency{}
	err := r.DB().QueryRowContext(ctx, query, name).Scan(
		&currency.ID,
		&currency.Name,
		&currency.CreatedAt,
		&currency.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return currency, nil
}

func (r *currencyRepository) List(ctx context.Context) ([]models.Currency, error) {
	query := `
		SELECT id, name, created_at, updated_at
		FROM currencies
		ORDER BY name ASC`

	rows, err := r.DB().QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var currencies []models.Currency
	for rows.Next() {
		var currency models.Currency
		if err := rows.Scan(
			&currency.ID,
			&currency.Name,
			&currency.CreatedAt,
			&currency.UpdatedAt,
		); err != nil {
			return nil, err
		}
		currencies = append(currencies, currency)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return currencies, nil
}
