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

func TestZoneRepository_Create(t *testing.T) {
	tc := integration.NewTestContext(t)

	tests := []struct {
		name    string
		input   models.Zone
		wantErr error
	}{
		{
			name: "Success - Create Zone",
			input: models.Zone{
				Name:     "test-zone",
				Timezone: "UTC",
			},
			wantErr: nil,
		},
		{
			name: "Error - Duplicate Name",
			input: models.Zone{
				Name:     "test-zone",
				Timezone: "UTC",
			},
			wantErr: repository.ErrConflict,
		},
		{
			name: "Error - Invalid Timezone",
			input: models.Zone{
				Name:     "test-zone-2",
				Timezone: "INVALID",
			},
			wantErr: repository.ErrInvalidTimezone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tc.ZoneRepo.Create(context.Background(), &tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Zero(t, tt.input.ID)
				return
			}

			require.NoError(t, err)
			require.NotEqual(t, uuid.Nil, tt.input.ID)
			require.False(t, tt.input.CreatedAt.IsZero())
			require.False(t, tt.input.UpdatedAt.IsZero())
			require.Equal(t, tt.input.Name, tt.input.Name)
			require.Equal(t, tt.input.Timezone, tt.input.Timezone)

			// Verify zone was created in database
			var exists bool
			err = tc.DB.QueryRowContext(context.Background(),
				"SELECT EXISTS(SELECT 1 FROM zones WHERE id = $1 AND name = $2 AND timezone = $3)",
				tt.input.ID, tt.input.Name, tt.input.Timezone).Scan(&exists)
			require.NoError(t, err)
			require.True(t, exists, "Zone record should exist in database")
		})
	}
}

func TestZoneRepository_Update(t *testing.T) {
	tc := integration.NewTestContext(t)

	// Create initial zone
	zone := &models.Zone{
		Name:     "test-zone",
		Timezone: "UTC",
	}
	require.NoError(t, tc.ZoneRepo.Create(context.Background(), zone))

	// Create another zone for duplicate name test
	otherZone := &models.Zone{
		Name:     "other-zone",
		Timezone: "UTC",
	}
	require.NoError(t, tc.ZoneRepo.Create(context.Background(), otherZone))

	tests := []struct {
		name    string
		input   models.Zone
		wantErr error
	}{
		{
			name: "Success - Update Zone",
			input: models.Zone{
				ID:       zone.ID,
				Name:     "updated-zone",
				Timezone: "America/New_York",
			},
			wantErr: nil,
		},
		{
			name: "Error - Non-existent ID",
			input: models.Zone{
				ID:       uuid.New(),
				Name:     "non-existent",
				Timezone: "UTC",
			},
			wantErr: repository.ErrNotFound,
		},
		{
			name: "Error - Duplicate Name",
			input: models.Zone{
				ID:       zone.ID,
				Name:     otherZone.Name,
				Timezone: "UTC",
			},
			wantErr: repository.ErrConflict,
		},
		{
			name: "Error - Invalid Timezone",
			input: models.Zone{
				ID:       zone.ID,
				Name:     "test-zone-3",
				Timezone: "INVALID",
			},
			wantErr: repository.ErrInvalidTimezone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tc.ZoneRepo.Update(context.Background(), &tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.False(t, tt.input.UpdatedAt.IsZero())
			require.Equal(t, tt.input.Name, tt.input.Name)
			require.Equal(t, tt.input.Timezone, tt.input.Timezone)

			// Verify zone was updated in database
			var exists bool
			err = tc.DB.QueryRowContext(context.Background(),
				"SELECT EXISTS(SELECT 1 FROM zones WHERE id = $1 AND name = $2 AND timezone = $3)",
				tt.input.ID, tt.input.Name, tt.input.Timezone).Scan(&exists)
			require.NoError(t, err)
			require.True(t, exists, "Updated zone record should exist in database")
		})
	}
}

func TestZoneRepository_Delete(t *testing.T) {
	tc := integration.NewTestContext(t)

	tests := []struct {
		name    string
		setup   func() uuid.UUID
		wantErr error
	}{
		{
			name: "Success",
			setup: func() uuid.UUID {
				zone := &models.Zone{
					Name:     "test-zone",
					Timezone: "UTC",
				}
				err := tc.ZoneRepo.Create(context.Background(), zone)
				require.NoError(t, err)
				return zone.ID
			},
		},
		{
			name: "Error - Non-existent ID",
			setup: func() uuid.UUID {
				return uuid.New()
			},
			wantErr: repository.ErrNotFound,
		},
		{
			name: "Error - Zone With Spot Prices",
			setup: func() uuid.UUID {
				// Create a zone
				zone := &models.Zone{
					Name:     "test-zone-with-prices",
					Timezone: "UTC",
				}
				err := tc.ZoneRepo.Create(context.Background(), zone)
				require.NoError(t, err)

				// Get the existing EUR currency
				currency, err := tc.CurrencyRepo.GetByName(context.Background(), "EUR")
				require.NoError(t, err)

				// Create a spot price for this zone
				spotPrice := &models.SpotPrice{
					ZoneID:     zone.ID,
					CurrencyID: currency.ID,
					Timestamp:  time.Now().UTC(),
					Price:      100.0,
				}
				err = tc.SpotPriceRepo.Create(context.Background(), spotPrice)
				require.NoError(t, err)

				return zone.ID
			},
			wantErr: repository.ErrHasAssociatedRecords,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.setup()
			err := tc.ZoneRepo.Delete(context.Background(), id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)

			// Verify zone was deleted
			var exists bool
			err = tc.DB.QueryRowContext(context.Background(),
				"SELECT EXISTS(SELECT 1 FROM zones WHERE id = $1)",
				id).Scan(&exists)
			require.NoError(t, err)
			require.False(t, exists, "Zone record should not exist in database after deletion")
		})
	}
}

func TestZoneRepository_GetByID(t *testing.T) {
	tc := integration.NewTestContext(t)

	// Create a test zone
	zone := &models.Zone{
		Name:     "test-zone",
		Timezone: "UTC",
	}
	require.NoError(t, tc.ZoneRepo.Create(context.Background(), zone))

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr error
	}{
		{
			name:    "Success - Get Zone",
			id:      zone.ID,
			wantErr: nil,
		},
		{
			name:    "Error - Non-existent ID",
			id:      uuid.New(),
			wantErr: repository.ErrNotFound,
		},
		{
			name:    "Error - Zero UUID",
			id:      uuid.Nil,
			wantErr: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tc.ZoneRepo.GetByID(context.Background(), tt.id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, result)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, zone.ID, result.ID)
			require.Equal(t, zone.Name, result.Name)
			require.Equal(t, zone.Timezone, result.Timezone)
			require.False(t, result.CreatedAt.IsZero())
			require.False(t, result.UpdatedAt.IsZero())
		})
	}
}

func TestZoneRepository_GetByName(t *testing.T) {
	tc := integration.NewTestContext(t)

	// Create zone to get
	zone := &models.Zone{
		Name:     "test-zone",
		Timezone: "UTC",
	}
	require.NoError(t, tc.ZoneRepo.Create(context.Background(), zone))

	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{
			name:    "Success - Get Zone By Name",
			input:   zone.Name,
			wantErr: nil,
		},
		{
			name:    "Error - Non-existent Name",
			input:   "non-existent",
			wantErr: repository.ErrNotFound,
		},
		{
			name:    "Error - Empty Name",
			input:   "",
			wantErr: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tc.ZoneRepo.GetByName(context.Background(), tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, result)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, zone.ID, result.ID)
			require.Equal(t, zone.Name, result.Name)
			require.Equal(t, zone.Timezone, result.Timezone)
			require.False(t, result.CreatedAt.IsZero())
			require.False(t, result.UpdatedAt.IsZero())

			// Verify zone exists in database
			var exists bool
			err = tc.DB.QueryRowContext(context.Background(),
				"SELECT EXISTS(SELECT 1 FROM zones WHERE id = $1 AND name = $2 AND timezone = $3)",
				result.ID, result.Name, result.Timezone).Scan(&exists)
			require.NoError(t, err)
			require.True(t, exists, "Zone record should exist in database")
		})
	}
}

func TestZoneRepository_List(t *testing.T) {
	tc := integration.NewTestContext(t)

	// Clean up existing zones
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM zones")
	require.NoError(t, err)

	// Create test zones
	zones := []models.Zone{
		{Name: "test-zone-1", Timezone: "UTC"},
		{Name: "test-zone-2", Timezone: "America/New_York"},
		{Name: "test-zone-3", Timezone: "Europe/London"},
	}

	for i := range zones {
		require.NoError(t, tc.ZoneRepo.Create(context.Background(), &zones[i]))
	}

	tests := []struct {
		name      string
		filter    repository.ZoneFilter
		wantCount int
		checkFunc func(t *testing.T, results []models.Zone)
	}{
		{
			name:      "Success - List All Zones",
			filter:    repository.ZoneFilter{},
			wantCount: len(zones),
			checkFunc: func(t *testing.T, results []models.Zone) {
				require.Len(t, results, len(zones))

				// Create maps for easy lookup
				expectedZones := make(map[string]models.Zone)
				for _, z := range zones {
					expectedZones[z.Name] = z
				}

				// Verify each returned zone
				for _, result := range results {
					expected, ok := expectedZones[result.Name]
					require.True(t, ok, "Unexpected zone in results: %s", result.Name)
					require.Equal(t, expected.Name, result.Name)
					require.Equal(t, expected.Timezone, result.Timezone)
					require.NotEqual(t, uuid.Nil, result.ID)
					require.False(t, result.CreatedAt.IsZero())
					require.False(t, result.UpdatedAt.IsZero())
				}
			},
		},
		{
			name: "Success - Filter By Name",
			filter: repository.ZoneFilter{
				Search: &[]string{"test-zone-1"}[0],
			},
			wantCount: 1,
			checkFunc: func(t *testing.T, results []models.Zone) {
				require.Len(t, results, 1)
				require.Equal(t, "test-zone-1", results[0].Name)
			},
		},
		{
			name: "Success - Order By Name DESC",
			filter: repository.ZoneFilter{
				OrderBy:   "name",
				OrderDesc: true,
			},
			wantCount: len(zones),
			checkFunc: func(t *testing.T, results []models.Zone) {
				require.Len(t, results, len(zones))
				// Verify descending order
				for i := 1; i < len(results); i++ {
					require.True(t, results[i-1].Name > results[i].Name)
				}
			},
		},
		{
			name: "Success - Pagination",
			filter: repository.ZoneFilter{
				Limit:  &[]int{2}[0],
				Offset: &[]int{1}[0],
			},
			wantCount: 2,
			checkFunc: func(t *testing.T, results []models.Zone) {
				require.Len(t, results, 2)
				// Verify we got the correct page
				require.Equal(t, "test-zone-2", results[0].Name)
				require.Equal(t, "test-zone-3", results[1].Name)
			},
		},
		{
			name: "Success - Empty Results",
			filter: repository.ZoneFilter{
				Search: &[]string{"non-existent"}[0],
			},
			wantCount: 0,
			checkFunc: func(t *testing.T, results []models.Zone) {
				require.Empty(t, results)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := tc.ZoneRepo.List(context.Background(), tt.filter)
			require.NoError(t, err)
			require.Len(t, results, tt.wantCount)

			if tt.checkFunc != nil {
				tt.checkFunc(t, results)
			}
		})
	}
}
