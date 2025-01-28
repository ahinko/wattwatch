package postgres_test

import (
	"context"
	"testing"
	"time"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"
	"wattwatch/internal/repository/postgres/integration"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCurrencyRepository_Create(t *testing.T) {
	tc := integration.NewTestContext(t)

	tests := []struct {
		name    string
		input   models.Currency
		wantErr bool
		errType error
	}{
		{
			name: "Success",
			input: models.Currency{
				Name: "USD",
			},
		},
		{
			name: "Duplicate Name",
			input: models.Currency{
				Name: "USD",
			},
			wantErr: true,
			errType: repository.ErrConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tc.CurrencyRepo.Create(context.Background(), &tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
				require.NotEqual(t, uuid.Nil, tt.input.ID)
				require.False(t, tt.input.CreatedAt.IsZero())
				require.False(t, tt.input.UpdatedAt.IsZero())

				// Verify creation
				var exists bool
				err = tc.DB.QueryRowContext(context.Background(),
					"SELECT EXISTS(SELECT 1 FROM currencies WHERE id = $1 AND name = $2)",
					tt.input.ID, tt.input.Name).Scan(&exists)
				require.NoError(t, err)
				require.True(t, exists)
			}
		})
	}
}

func TestCurrencyRepository_Update(t *testing.T) {
	tc := integration.NewTestContext(t)

	// Clean up any existing currencies
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM currencies")
	require.NoError(t, err)

	// Create initial currency
	currency := &models.Currency{
		Name: "USD",
	}
	require.NoError(t, tc.CurrencyRepo.Create(context.Background(), currency))

	// Create another currency for duplicate name test
	otherCurrency := &models.Currency{
		Name: "EUR",
	}
	require.NoError(t, tc.CurrencyRepo.Create(context.Background(), otherCurrency))

	tests := []struct {
		name    string
		input   models.Currency
		wantErr bool
		errType error
	}{
		{
			name: "Success",
			input: models.Currency{
				ID:   currency.ID,
				Name: "GBP",
			},
		},
		{
			name: "Non-existent ID",
			input: models.Currency{
				ID:   uuid.New(),
				Name: "NOK",
			},
			wantErr: true,
			errType: repository.ErrNotFound,
		},
		{
			name: "Duplicate Name",
			input: models.Currency{
				ID:   currency.ID,
				Name: otherCurrency.Name,
			},
			wantErr: true,
			errType: repository.ErrConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tc.CurrencyRepo.Update(context.Background(), &tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
				require.False(t, tt.input.UpdatedAt.IsZero())

				// Verify update
				var exists bool
				err = tc.DB.QueryRowContext(context.Background(),
					"SELECT EXISTS(SELECT 1 FROM currencies WHERE id = $1 AND name = $2)",
					tt.input.ID, tt.input.Name).Scan(&exists)
				require.NoError(t, err)
				require.True(t, exists)
			}
		})
	}
}

func TestCurrencyRepository_Delete(t *testing.T) {
	tc := integration.NewTestContext(t)

	// Clean up any existing currencies
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM currencies")
	require.NoError(t, err)

	// Create currency to delete
	currency := &models.Currency{
		Name: "USD",
	}
	require.NoError(t, tc.CurrencyRepo.Create(context.Background(), currency))

	// Create currency with associated spot prices
	currencyWithSpotPrices := &models.Currency{
		Name: "EUR",
	}
	require.NoError(t, tc.CurrencyRepo.Create(context.Background(), currencyWithSpotPrices))

	// Create spot price for the currency
	zone := tc.CreateTestZone("test-zone", "UTC")
	spotPrice := &models.SpotPrice{
		Timestamp:  time.Now().UTC(),
		ZoneID:     zone.ID,
		CurrencyID: currencyWithSpotPrices.ID,
		Price:      100.50,
	}
	require.NoError(t, tc.SpotPriceRepo.Create(context.Background(), spotPrice))

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr bool
		errType error
	}{
		{
			name: "Success",
			id:   currency.ID,
		},
		{
			name:    "Non-existent ID",
			id:      uuid.New(),
			wantErr: true,
			errType: repository.ErrNotFound,
		},
		{
			name:    "Currency with Spot Prices",
			id:      currencyWithSpotPrices.ID,
			wantErr: true,
			errType: repository.ErrHasAssociatedRecords,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tc.CurrencyRepo.Delete(context.Background(), tt.id)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)

				// Verify deletion
				var exists bool
				err = tc.DB.QueryRowContext(context.Background(),
					"SELECT EXISTS(SELECT 1 FROM currencies WHERE id = $1)",
					tt.id).Scan(&exists)
				require.NoError(t, err)
				require.False(t, exists)
			}
		})
	}
}

func TestCurrencyRepository_GetByID(t *testing.T) {
	tc := integration.NewTestContext(t)

	// Create currency to get
	currency := &models.Currency{
		Name: "USD",
	}
	require.NoError(t, tc.CurrencyRepo.Create(context.Background(), currency))

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr bool
		errType error
	}{
		{
			name: "Success",
			id:   currency.ID,
		},
		{
			name:    "Non-existent ID",
			id:      uuid.New(),
			wantErr: true,
			errType: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tc.CurrencyRepo.GetByID(context.Background(), tt.id)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, currency.Name, result.Name)
			}
		})
	}
}

func TestCurrencyRepository_GetByName(t *testing.T) {
	tc := integration.NewTestContext(t)

	// Create currency to get
	currency := &models.Currency{
		Name: "USD",
	}
	require.NoError(t, tc.CurrencyRepo.Create(context.Background(), currency))

	tests := []struct {
		name    string
		input   string
		wantErr bool
		errType error
	}{
		{
			name:  "Success",
			input: currency.Name,
		},
		{
			name:    "Non-existent Name",
			input:   "NOK",
			wantErr: true,
			errType: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tc.CurrencyRepo.GetByName(context.Background(), tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, currency.Name, result.Name)
			}
		})
	}
}

func TestCurrencyRepository_List(t *testing.T) {
	tc := integration.NewTestContext(t)

	// Clean up any existing currencies
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM currencies")
	require.NoError(t, err)

	// Create test currencies
	currencies := []models.Currency{
		{Name: "USD"},
		{Name: "EUR"},
		{Name: "GBP"},
		{Name: "JPY"},
	}

	for i := range currencies {
		require.NoError(t, tc.CurrencyRepo.Create(context.Background(), &currencies[i]))
	}

	// Test listing
	results, err := tc.CurrencyRepo.List(context.Background())
	require.NoError(t, err)
	require.Len(t, results, len(currencies))

	// Verify order (should be alphabetical by name)
	for i := 1; i < len(results); i++ {
		require.True(t, results[i-1].Name < results[i].Name)
	}
}
