package handlers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"wattwatch/internal/api/handlers"
	"wattwatch/internal/api/middleware"
	"wattwatch/internal/models"
	"wattwatch/internal/repository/postgres"
	"wattwatch/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type spotPriceTest struct {
	name       string
	setupFunc  func(*testutil.TestContext) string
	input      interface{}
	wantStatus int
	wantErr    bool
}

func TestSpotPriceHandler_ListSpotPrices(t *testing.T) {
	tc := testutil.NewTestContext(t)

	// Create test user
	user := tc.CreateTestUser("user", "user@test.com", "password123", false)
	token := tc.GetTestJWT(user.ID)

	// Get test zone and currency
	var zoneID, currencyID uuid.UUID
	var zoneName, currencyName string
	err := tc.DB.QueryRow(`SELECT id, name FROM zones WHERE name = 'SE1'`).Scan(&zoneID, &zoneName)
	require.NoError(t, err)
	err = tc.DB.QueryRow(`SELECT id, name FROM currencies WHERE name = 'EUR'`).Scan(&currencyID, &currencyName)
	require.NoError(t, err)

	// Insert test data
	now := time.Now().UTC()
	spotPrice1ID := uuid.New()
	spotPrice2ID := uuid.New()
	spotPrice3ID := uuid.New()
	_, err = tc.DB.Exec(`
		INSERT INTO spot_prices (id, zone_id, currency_id, price, timestamp) VALUES
		($1, $2, $3, 50.5, $4),
		($5, $6, $7, 60.5, $8),
		($9, $10, $11, 70.5, $12)
	`,
		spotPrice1ID, zoneID, currencyID, now,
		spotPrice2ID, zoneID, currencyID, now.Add(time.Hour),
		spotPrice3ID, zoneID, currencyID, now.Add(8*24*time.Hour), // Outside 7-day range
	)
	require.NoError(t, err)

	handler := handlers.NewSpotPriceHandler(
		postgres.NewSpotPriceRepository(tc.DB),
		postgres.NewZoneRepository(tc.DB),
		postgres.NewCurrencyRepository(tc.DB),
	)
	router := gin.New()
	authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
	router.Use(authMiddleware.AuthRequired())
	router.GET("/spot-prices", handler.ListSpotPrices)

	tests := []struct {
		name       string
		query      string
		wantStatus int
		wantCount  int
		wantErr    bool
	}{
		{
			name: "Valid Request",
			query: fmt.Sprintf("zone=%s&currency=%s&start_time=%s&end_time=%s",
				zoneName,
				currencyName,
				now.Format(time.RFC3339),
				now.Add(7*24*time.Hour).Format(time.RFC3339)),
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name: "Missing Zone",
			query: fmt.Sprintf("currency=%s&start_time=%s&end_time=%s",
				currencyName,
				now.Format(time.RFC3339),
				now.Add(7*24*time.Hour).Format(time.RFC3339)),
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "Missing Currency",
			query: fmt.Sprintf("zone=%s&start_time=%s&end_time=%s",
				zoneName,
				now.Format(time.RFC3339),
				now.Add(7*24*time.Hour).Format(time.RFC3339)),
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "Missing Start Time",
			query: fmt.Sprintf("zone=%s&currency=%s&end_time=%s",
				zoneName,
				currencyName,
				now.Add(7*24*time.Hour).Format(time.RFC3339)),
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "Missing End Time",
			query: fmt.Sprintf("zone=%s&currency=%s&start_time=%s",
				zoneName,
				currencyName,
				now.Format(time.RFC3339)),
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "Invalid Zone",
			query: fmt.Sprintf("zone=INVALID&currency=%s&start_time=%s&end_time=%s",
				currencyName,
				now.Format(time.RFC3339),
				now.Add(7*24*time.Hour).Format(time.RFC3339)),
			wantStatus: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name: "Invalid Currency",
			query: fmt.Sprintf("zone=%s&currency=INVALID&start_time=%s&end_time=%s",
				zoneName,
				now.Format(time.RFC3339),
				now.Add(7*24*time.Hour).Format(time.RFC3339)),
			wantStatus: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name: "Invalid Date Range (> 7 days)",
			query: fmt.Sprintf("zone=%s&currency=%s&start_time=%s&end_time=%s",
				zoneName,
				currencyName,
				now.Format(time.RFC3339),
				now.Add(8*24*time.Hour).Format(time.RFC3339)),
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "End Time Before Start Time",
			query: fmt.Sprintf("zone=%s&currency=%s&start_time=%s&end_time=%s",
				zoneName,
				currencyName,
				now.Format(time.RFC3339),
				now.Add(-24*time.Hour).Format(time.RFC3339)),
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "Descending Order",
			query: fmt.Sprintf("zone=%s&currency=%s&start_time=%s&end_time=%s&order_desc=true",
				zoneName,
				currencyName,
				now.Format(time.RFC3339),
				now.Add(7*24*time.Hour).Format(time.RFC3339)),
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/spot-prices?"+tt.query, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if !tt.wantErr {
				var spotPrices []models.SpotPrice
				err = json.Unmarshal(w.Body.Bytes(), &spotPrices)
				require.NoError(t, err)
				assert.Equal(t, tt.wantCount, len(spotPrices))

				if tt.name == "Descending Order" {
					// Verify descending order
					for i := 1; i < len(spotPrices); i++ {
						assert.True(t, spotPrices[i-1].Timestamp.After(spotPrices[i].Timestamp))
					}
				} else {
					// Verify ascending order
					for i := 1; i < len(spotPrices); i++ {
						assert.True(t, spotPrices[i-1].Timestamp.Before(spotPrices[i].Timestamp))
					}
				}
			}
		})
	}
}

func TestSpotPriceHandler_CreateSpotPrices(t *testing.T) {
	tests := []spotPriceTest{
		{
			name: "Valid Single Spot Price",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateSpotPricesRequest{
				SpotPrices: []models.CreateSpotPriceRequest{
					{
						Timestamp:  time.Now().UTC(),
						ZoneID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual zone ID
						CurrencyID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual currency ID
						Price:      42.50,
					},
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "Valid Multiple Spot Prices",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateSpotPricesRequest{
				SpotPrices: []models.CreateSpotPriceRequest{
					{
						Timestamp:  time.Now().UTC(),
						ZoneID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual zone ID
						CurrencyID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual currency ID
						Price:      42.50,
					},
					{
						Timestamp:  time.Now().UTC().Add(time.Hour),
						ZoneID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual zone ID
						CurrencyID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual currency ID
						Price:      43.50,
					},
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "Update Existing Spot Price",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)

				// Get test zone and currency IDs
				var zoneID, currencyID uuid.UUID
				err := tc.DB.QueryRow(`SELECT id FROM zones WHERE name = 'SE1'`).Scan(&zoneID)
				require.NoError(t, err)
				err = tc.DB.QueryRow(`SELECT id FROM currencies WHERE name = 'EUR'`).Scan(&currencyID)
				require.NoError(t, err)

				// Insert existing spot price
				timestamp := time.Now().UTC()
				_, err = tc.DB.Exec(`
					INSERT INTO spot_prices (id, timestamp, zone_id, currency_id, price, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $6)
				`, uuid.New(), timestamp, zoneID, currencyID, 42.50, time.Now())
				require.NoError(t, err)

				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateSpotPricesRequest{
				SpotPrices: []models.CreateSpotPriceRequest{
					{
						Timestamp:  time.Now().UTC(),
						ZoneID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual zone ID
						CurrencyID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual currency ID
						Price:      50.00,                                                  // Updated price
					},
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "Mixed Create and Update",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)

				// Get test zone and currency IDs
				var zoneID, currencyID uuid.UUID
				err := tc.DB.QueryRow(`SELECT id FROM zones WHERE name = 'SE1'`).Scan(&zoneID)
				require.NoError(t, err)
				err = tc.DB.QueryRow(`SELECT id FROM currencies WHERE name = 'EUR'`).Scan(&currencyID)
				require.NoError(t, err)

				// Insert existing spot price
				timestamp := time.Now().UTC()
				_, err = tc.DB.Exec(`
					INSERT INTO spot_prices (id, timestamp, zone_id, currency_id, price, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $6)
				`, uuid.New(), timestamp, zoneID, currencyID, 42.50, time.Now())
				require.NoError(t, err)

				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateSpotPricesRequest{
				SpotPrices: []models.CreateSpotPriceRequest{
					{
						Timestamp:  time.Now().UTC(),
						ZoneID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual zone ID
						CurrencyID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual currency ID
						Price:      50.00,                                                  // Updated price
					},
					{
						Timestamp:  time.Now().UTC().Add(time.Hour),
						ZoneID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual zone ID
						CurrencyID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual currency ID
						Price:      43.50,                                                  // New price
					},
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "Not Authorized (Regular User)",
			setupFunc: func(tc *testutil.TestContext) string {
				user := tc.CreateTestUser("user", "user@test.com", "password123", false)
				return tc.GetTestJWT(user.ID)
			},
			input: models.CreateSpotPricesRequest{
				SpotPrices: []models.CreateSpotPriceRequest{
					{
						Timestamp:  time.Now().UTC(),
						ZoneID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual zone ID
						CurrencyID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual currency ID
						Price:      42.50,
					},
				},
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
		},
		{
			name: "Not Authenticated",
			setupFunc: func(tc *testutil.TestContext) string {
				return ""
			},
			input: models.CreateSpotPricesRequest{
				SpotPrices: []models.CreateSpotPriceRequest{
					{
						Timestamp:  time.Now().UTC(),
						ZoneID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual zone ID
						CurrencyID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual currency ID
						Price:      42.50,
					},
				},
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name: "Invalid Zone ID",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateSpotPricesRequest{
				SpotPrices: []models.CreateSpotPriceRequest{
					{
						Timestamp:  time.Now().UTC(),
						ZoneID:     uuid.New(),                                             // Non-existent zone ID
						CurrencyID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual currency ID
						Price:      42.50,
					},
				},
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "Invalid Currency ID",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateSpotPricesRequest{
				SpotPrices: []models.CreateSpotPriceRequest{
					{
						Timestamp:  time.Now().UTC(),
						ZoneID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual zone ID
						CurrencyID: uuid.New(),                                             // Non-existent currency ID
						Price:      42.50,
					},
				},
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "Invalid Price (Negative)",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateSpotPricesRequest{
				SpotPrices: []models.CreateSpotPriceRequest{
					{
						Timestamp:  time.Now().UTC(),
						ZoneID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual zone ID
						CurrencyID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Will be replaced with actual currency ID
						Price:      -42.50,
					},
				},
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "Empty Spot Prices",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateSpotPricesRequest{
				SpotPrices: []models.CreateSpotPriceRequest{},
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			// Get test zone and currency IDs
			var zoneID, currencyID uuid.UUID
			err := tc.DB.QueryRow(`SELECT id FROM zones WHERE name = 'SE1'`).Scan(&zoneID)
			require.NoError(t, err)
			err = tc.DB.QueryRow(`SELECT id FROM currencies WHERE name = 'EUR'`).Scan(&currencyID)
			require.NoError(t, err)

			// Replace placeholder IDs with actual IDs
			if req, ok := tt.input.(models.CreateSpotPricesRequest); ok {
				for i := range req.SpotPrices {
					if req.SpotPrices[i].ZoneID == uuid.MustParse("00000000-0000-0000-0000-000000000001") {
						req.SpotPrices[i].ZoneID = zoneID
					}
					if req.SpotPrices[i].CurrencyID == uuid.MustParse("00000000-0000-0000-0000-000000000001") {
						req.SpotPrices[i].CurrencyID = currencyID
					}
				}
				tt.input = req
			}

			token := ""
			if tt.setupFunc != nil {
				token = tt.setupFunc(tc)
			}

			handler := handlers.NewSpotPriceHandler(
				postgres.NewSpotPriceRepository(tc.DB),
				postgres.NewZoneRepository(tc.DB),
				postgres.NewCurrencyRepository(tc.DB),
			)
			router := gin.New()
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router.Use(authMiddleware.AuthRequired())
			router.POST("/spot-prices", authMiddleware.AdminRequired(), handler.CreateSpotPrices)

			body, err := json.Marshal(tt.input)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/spot-prices", bytes.NewBuffer(body))
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if !tt.wantErr && tt.wantStatus == http.StatusCreated {
				var spotPrices []models.SpotPrice
				err = json.Unmarshal(w.Body.Bytes(), &spotPrices)
				require.NoError(t, err)
				assert.Equal(t, len(tt.input.(models.CreateSpotPricesRequest).SpotPrices), len(spotPrices))

				// Verify spot prices were created/updated
				for i, sp := range spotPrices {
					// First verify only one record exists
					var count int
					err := tc.DB.QueryRow(`
						SELECT COUNT(*) 
						FROM spot_prices 
						WHERE timestamp = $1 AND zone_id = $2 AND currency_id = $3`,
						sp.Timestamp, sp.ZoneID, sp.CurrencyID).Scan(&count)
					require.NoError(t, err)
					assert.Equal(t, 1, count)

					// Then get the actual record to verify the price
					var price float64
					err = tc.DB.QueryRow(`
						SELECT price 
						FROM spot_prices 
						WHERE timestamp = $1 AND zone_id = $2 AND currency_id = $3`,
						sp.Timestamp, sp.ZoneID, sp.CurrencyID).Scan(&price)
					require.NoError(t, err)

					// Verify the data matches
					inputSP := tt.input.(models.CreateSpotPricesRequest).SpotPrices[i]
					assert.Equal(t, inputSP.Timestamp.UTC(), sp.Timestamp.UTC())
					assert.Equal(t, inputSP.ZoneID, sp.ZoneID)
					assert.Equal(t, inputSP.CurrencyID, sp.CurrencyID)
					assert.Equal(t, inputSP.Price, price)
				}
			}
		})
	}
}
