package postgres_test

import (
	"context"
	"testing"
	"time"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"
	"wattwatch/internal/repository/postgres"
	"wattwatch/internal/testutil"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSpotPriceRepository_Create(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewSpotPriceRepository(tc.DB)

	// Create test zone and currency
	zone := tc.CreateTestZone("test-zone", "UTC")
	currency := tc.CreateTestCurrency("USD")

	timestamp := time.Now().UTC()

	tests := []struct {
		name      string
		input     models.SpotPrice
		wantErr   bool
		checkFunc func(t *testing.T, sp *models.SpotPrice)
	}{
		{
			name: "Success",
			input: models.SpotPrice{
				Timestamp:  timestamp,
				ZoneID:     zone.ID,
				CurrencyID: currency.ID,
				Price:      100.50,
			},
		},
		{
			name: "Update On Duplicate",
			input: models.SpotPrice{
				Timestamp:  timestamp,
				ZoneID:     zone.ID,
				CurrencyID: currency.ID,
				Price:      150.75, // Different price
			},
			checkFunc: func(t *testing.T, sp *models.SpotPrice) {
				require.Equal(t, 150.75, sp.Price)
			},
		},
		{
			name: "Invalid Zone ID",
			input: models.SpotPrice{
				Timestamp:  timestamp,
				ZoneID:     uuid.New(),
				CurrencyID: currency.ID,
				Price:      100.50,
			},
			wantErr: true,
		},
		{
			name: "Invalid Currency ID",
			input: models.SpotPrice{
				Timestamp:  timestamp,
				ZoneID:     zone.ID,
				CurrencyID: uuid.New(),
				Price:      100.50,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(context.Background(), &tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotEqual(t, uuid.Nil, tt.input.ID)
				require.False(t, tt.input.CreatedAt.IsZero())
				require.False(t, tt.input.UpdatedAt.IsZero())
				if tt.checkFunc != nil {
					tt.checkFunc(t, &tt.input)
				}
			}
		})
	}
}

func TestSpotPriceRepository_CreateBatch(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewSpotPriceRepository(tc.DB)

	// Create test zone and currency
	zone := tc.CreateTestZone("test-zone", "UTC")
	currency := tc.CreateTestCurrency("USD")

	baseTime := time.Now().UTC()

	tests := []struct {
		name      string
		input     []models.SpotPrice
		wantErr   bool
		checkFunc func(t *testing.T, input []models.SpotPrice)
	}{
		{
			name: "Success - New Records",
			input: []models.SpotPrice{
				{
					Timestamp:  baseTime,
					ZoneID:     zone.ID,
					CurrencyID: currency.ID,
					Price:      100.50,
				},
				{
					Timestamp:  baseTime.Add(time.Hour),
					ZoneID:     zone.ID,
					CurrencyID: currency.ID,
					Price:      101.50,
				},
			},
			checkFunc: func(t *testing.T, input []models.SpotPrice) {
				for _, sp := range input {
					require.NotEqual(t, uuid.Nil, sp.ID)
					require.False(t, sp.CreatedAt.IsZero())
					require.False(t, sp.UpdatedAt.IsZero())
				}
			},
		},
		{
			name: "Success - Mixed Create and Update",
			input: []models.SpotPrice{
				{
					Timestamp:  baseTime,
					ZoneID:     zone.ID,
					CurrencyID: currency.ID,
					Price:      102.50, // Updated price
				},
				{
					Timestamp:  baseTime.Add(time.Hour * 2),
					ZoneID:     zone.ID,
					CurrencyID: currency.ID,
					Price:      103.50,
				},
			},
			checkFunc: func(t *testing.T, input []models.SpotPrice) {
				// First record should have updated price
				sp, err := repo.GetByID(context.Background(), input[0].ID)
				require.NoError(t, err)
				require.Equal(t, 102.50, sp.Price)
			},
		},
		{
			name:    "Success - Empty Batch",
			input:   []models.SpotPrice{},
			wantErr: false,
		},
		{
			name: "Error - Invalid Zone ID",
			input: []models.SpotPrice{
				{
					Timestamp:  baseTime.Add(time.Hour * 3),
					ZoneID:     uuid.New(),
					CurrencyID: currency.ID,
					Price:      104.50,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.CreateBatch(context.Background(), tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.checkFunc != nil {
					tt.checkFunc(t, tt.input)
				}
			}
		})
	}
}

func TestSpotPriceRepository_Update(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewSpotPriceRepository(tc.DB)

	// Create test zone and currency
	zone := tc.CreateTestZone("test-zone", "UTC")
	currency := tc.CreateTestCurrency("USD")

	// Create initial spot price
	sp := &models.SpotPrice{
		Timestamp:  time.Now().UTC(),
		ZoneID:     zone.ID,
		CurrencyID: currency.ID,
		Price:      100.50,
	}
	require.NoError(t, repo.Create(context.Background(), sp))

	tests := []struct {
		name    string
		input   models.SpotPrice
		wantErr bool
	}{
		{
			name: "Success",
			input: models.SpotPrice{
				ID:         sp.ID,
				Timestamp:  sp.Timestamp,
				ZoneID:     zone.ID,
				CurrencyID: currency.ID,
				Price:      101.50,
			},
		},
		{
			name: "Non-existent ID",
			input: models.SpotPrice{
				ID:         uuid.New(),
				Timestamp:  time.Now().UTC(),
				ZoneID:     zone.ID,
				CurrencyID: currency.ID,
				Price:      102.50,
			},
			wantErr: true,
		},
		{
			name: "Invalid Zone ID",
			input: models.SpotPrice{
				ID:         sp.ID,
				Timestamp:  sp.Timestamp,
				ZoneID:     uuid.New(),
				CurrencyID: currency.ID,
				Price:      103.50,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Update(context.Background(), &tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.False(t, tt.input.UpdatedAt.IsZero())

				// Verify update
				updated, err := repo.GetByID(context.Background(), tt.input.ID)
				require.NoError(t, err)
				require.Equal(t, tt.input.Price, updated.Price)
			}
		})
	}
}

func TestSpotPriceRepository_Delete(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewSpotPriceRepository(tc.DB)

	// Create test zone and currency
	zone := tc.CreateTestZone("test-zone", "UTC")
	currency := tc.CreateTestCurrency("USD")

	// Create spot price to delete
	sp := &models.SpotPrice{
		Timestamp:  time.Now().UTC(),
		ZoneID:     zone.ID,
		CurrencyID: currency.ID,
		Price:      100.50,
	}
	require.NoError(t, repo.Create(context.Background(), sp))

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr bool
	}{
		{
			name: "Success",
			id:   sp.ID,
		},
		{
			name:    "Non-existent ID",
			id:      uuid.New(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Delete(context.Background(), tt.id)
			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, repository.ErrNotFound, err)
			} else {
				require.NoError(t, err)

				// Verify deletion
				_, err := repo.GetByID(context.Background(), tt.id)
				require.Error(t, err)
				require.Equal(t, repository.ErrNotFound, err)
			}
		})
	}
}

func TestSpotPriceRepository_List(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewSpotPriceRepository(tc.DB)

	// Create test zones and currencies
	zone1 := tc.CreateTestZone("test-zone-1", "UTC")
	zone2 := tc.CreateTestZone("test-zone-2", "UTC")
	currency1 := tc.CreateTestCurrency("DKK")
	currency2 := tc.CreateTestCurrency("AUD")

	// Create test data
	baseTime := time.Now().UTC()
	spotPrices := []models.SpotPrice{
		{
			Timestamp:  baseTime,
			ZoneID:     zone1.ID,
			CurrencyID: currency1.ID,
			Price:      100.50,
		},
		{
			Timestamp:  baseTime.Add(time.Hour),
			ZoneID:     zone1.ID,
			CurrencyID: currency1.ID,
			Price:      101.50,
		},
		{
			Timestamp:  baseTime.Add(time.Hour * 2),
			ZoneID:     zone2.ID,
			CurrencyID: currency1.ID,
			Price:      102.50,
		},
		{
			Timestamp:  baseTime.Add(time.Hour * 3),
			ZoneID:     zone2.ID,
			CurrencyID: currency2.ID,
			Price:      103.50,
		},
	}

	for i := range spotPrices {
		require.NoError(t, repo.Create(context.Background(), &spotPrices[i]))
	}

	tests := []struct {
		name      string
		filter    repository.SpotPriceFilter
		wantCount int
		checkFunc func(t *testing.T, results []models.SpotPrice)
	}{
		{
			name:      "No Filter",
			filter:    repository.SpotPriceFilter{},
			wantCount: 4,
		},
		{
			name: "Filter by Zone",
			filter: repository.SpotPriceFilter{
				ZoneID: &zone1.ID,
			},
			wantCount: 2,
			checkFunc: func(t *testing.T, results []models.SpotPrice) {
				for _, sp := range results {
					require.Equal(t, zone1.ID, sp.ZoneID)
				}
			},
		},
		{
			name: "Filter by Currency",
			filter: repository.SpotPriceFilter{
				CurrencyID: &currency1.ID,
			},
			wantCount: 3,
			checkFunc: func(t *testing.T, results []models.SpotPrice) {
				for _, sp := range results {
					require.Equal(t, currency1.ID, sp.CurrencyID)
				}
			},
		},
		{
			name: "Filter by Time Range",
			filter: repository.SpotPriceFilter{
				StartTime: &baseTime,
				EndTime:   &[]time.Time{baseTime.Add(time.Hour)}[0],
			},
			wantCount: 2,
		},
		{
			name: "Order by Price DESC",
			filter: repository.SpotPriceFilter{
				OrderBy:   "price",
				OrderDesc: true,
			},
			wantCount: 4,
			checkFunc: func(t *testing.T, results []models.SpotPrice) {
				for i := 1; i < len(results); i++ {
					require.GreaterOrEqual(t, results[i-1].Price, results[i].Price)
				}
			},
		},
		{
			name: "Pagination",
			filter: repository.SpotPriceFilter{
				Limit:  &[]int{2}[0],
				Offset: &[]int{1}[0],
			},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := repo.List(context.Background(), tt.filter)
			require.NoError(t, err)
			require.Len(t, results, tt.wantCount)

			if tt.checkFunc != nil {
				tt.checkFunc(t, results)
			}
		})
	}
}
