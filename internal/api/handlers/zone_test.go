package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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

type zoneTest struct {
	name       string
	setupFunc  func(*testutil.TestContext) string
	input      interface{}
	wantStatus int
	wantErr    bool
	errMsg     string
}

func TestZoneHandler_ListZones(t *testing.T) {
	tc := testutil.NewTestContext(t)

	handler := handlers.NewZoneHandler(postgres.NewZoneRepository(tc.DB))
	router := gin.New()
	router.GET("/zones", handler.ListZones)

	// Insert test data (in addition to defaults)
	zone1ID := uuid.New()
	zone2ID := uuid.New()
	_, err := tc.DB.Exec(`
		INSERT INTO zones (id, name, timezone) VALUES
		($1, 'TEST1', 'Europe/London'),
		($2, 'TEST2', 'Europe/Paris')
	`, zone1ID, zone2ID)
	require.NoError(t, err)

	// Test listing zones (should include defaults + inserted)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/zones", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var zones []models.Zone
	err = json.Unmarshal(w.Body.Bytes(), &zones)
	require.NoError(t, err)
	assert.Len(t, zones, 6) // 4 defaults + 2 inserted
	// Check they're ordered alphabetically
	assert.Equal(t, "SE1", zones[0].Name)
	assert.Equal(t, "SE2", zones[1].Name)
	assert.Equal(t, "SE3", zones[2].Name)
	assert.Equal(t, "SE4", zones[3].Name)
	assert.Equal(t, "TEST1", zones[4].Name)
	assert.Equal(t, "TEST2", zones[5].Name)
}

func TestZoneHandler_GetZone(t *testing.T) {
	tc := testutil.NewTestContext(t)

	handler := handlers.NewZoneHandler(postgres.NewZoneRepository(tc.DB))
	router := gin.New()
	router.GET("/zones/:id", handler.GetZone)

	// Insert test data
	zoneID := uuid.New()
	_, err := tc.DB.Exec(`
		INSERT INTO zones (id, name, timezone) VALUES ($1, 'TEST1', 'Europe/London')
	`, zoneID)
	require.NoError(t, err)

	// Test getting existing zone
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/zones/"+zoneID.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var zone models.Zone
	err = json.Unmarshal(w.Body.Bytes(), &zone)
	require.NoError(t, err)
	assert.Equal(t, zoneID, zone.ID)
	assert.Equal(t, "TEST1", zone.Name)
	assert.Equal(t, "Europe/London", zone.Timezone)

	// Test getting non-existent zone
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/zones/"+uuid.New().String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestZoneHandler_CreateZone(t *testing.T) {
	tests := []zoneTest{
		{
			name: "Valid Zone (Admin)",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateZoneRequest{
				Name:     "TEST1",
				Timezone: "Europe/London",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "Not Authorized (Regular User)",
			setupFunc: func(tc *testutil.TestContext) string {
				user := tc.CreateTestUser("user", "user@test.com", "password123", false)
				return tc.GetTestJWT(user.ID)
			},
			input: models.CreateZoneRequest{
				Name:     "TEST1",
				Timezone: "Europe/London",
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
		},
		{
			name: "Not Authenticated",
			setupFunc: func(tc *testutil.TestContext) string {
				return ""
			},
			input: models.CreateZoneRequest{
				Name:     "TEST1",
				Timezone: "Europe/London",
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name: "Missing Name",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateZoneRequest{
				Timezone: "Europe/London",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "Missing Timezone",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateZoneRequest{
				Name: "TEST1",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "Duplicate Zone",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateZoneRequest{
				Name:     "SE1", // Try to create existing default zone
				Timezone: "Europe/London",
			},
			wantStatus: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			token := ""
			if tt.setupFunc != nil {
				token = tt.setupFunc(tc)
			}

			handler := handlers.NewZoneHandler(postgres.NewZoneRepository(tc.DB))
			router := gin.New()
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router.Use(authMiddleware.AuthRequired())
			router.POST("/zones", authMiddleware.AdminRequired(), handler.CreateZone)

			body, err := json.Marshal(tt.input)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/zones", bytes.NewBuffer(body))
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if !tt.wantErr {
				var zone models.Zone
				err = json.Unmarshal(w.Body.Bytes(), &zone)
				require.NoError(t, err)
				assert.Equal(t, tt.input.(models.CreateZoneRequest).Name, zone.Name)
				assert.Equal(t, tt.input.(models.CreateZoneRequest).Timezone, zone.Timezone)
			}
		})
	}
}

func TestZoneHandler_UpdateZone(t *testing.T) {
	tests := []zoneTest{
		{
			name: "Valid Update (Admin)",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				_, err := tc.DB.Exec(`INSERT INTO zones (id, name, timezone) VALUES ($1, 'TEST1', 'Europe/London')`, uuid.New())
				require.NoError(t, err)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateZoneRequest{
				Name:     "TEST2",
				Timezone: "Europe/Paris",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Not Authorized (Regular User)",
			setupFunc: func(tc *testutil.TestContext) string {
				user := tc.CreateTestUser("user", "user@test.com", "password123", false)
				_, err := tc.DB.Exec(`INSERT INTO zones (id, name, timezone) VALUES ($1, 'TEST1', 'Europe/London')`, uuid.New())
				require.NoError(t, err)
				return tc.GetTestJWT(user.ID)
			},
			input: models.UpdateZoneRequest{
				Name:     "TEST2",
				Timezone: "Europe/Paris",
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
		},
		{
			name: "Not Authenticated",
			setupFunc: func(tc *testutil.TestContext) string {
				_, err := tc.DB.Exec(`INSERT INTO zones (id, name, timezone) VALUES ($1, 'TEST1', 'Europe/London')`, uuid.New())
				require.NoError(t, err)
				return ""
			},
			input: models.UpdateZoneRequest{
				Name:     "TEST2",
				Timezone: "Europe/Paris",
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name: "Missing Name",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				_, err := tc.DB.Exec(`INSERT INTO zones (id, name, timezone) VALUES ($1, 'TEST1', 'Europe/London')`, uuid.New())
				require.NoError(t, err)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateZoneRequest{
				Timezone: "Europe/Paris",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "Missing Timezone",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				_, err := tc.DB.Exec(`INSERT INTO zones (id, name, timezone) VALUES ($1, 'TEST1', 'Europe/London')`, uuid.New())
				require.NoError(t, err)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateZoneRequest{
				Name: "TEST2",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "Update to Existing Name",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				_, err := tc.DB.Exec(`INSERT INTO zones (id, name, timezone) VALUES ($1, 'TEST1', 'Europe/London')`, uuid.New())
				require.NoError(t, err)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateZoneRequest{
				Name:     "SE1", // Try to update to existing default zone
				Timezone: "Europe/Paris",
			},
			wantStatus: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			token := ""
			if tt.setupFunc != nil {
				token = tt.setupFunc(tc)
			}

			handler := handlers.NewZoneHandler(postgres.NewZoneRepository(tc.DB))
			router := gin.New()
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router.Use(authMiddleware.AuthRequired())
			router.PUT("/zones/:id", authMiddleware.AdminRequired(), handler.UpdateZone)

			// Get an existing zone ID
			var zoneID uuid.UUID
			err := tc.DB.QueryRow(`SELECT id FROM zones WHERE name = 'TEST1'`).Scan(&zoneID)
			require.NoError(t, err)

			body, err := json.Marshal(tt.input)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", "/zones/"+zoneID.String(), bytes.NewBuffer(body))
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if !tt.wantErr {
				var zone models.Zone
				err = json.Unmarshal(w.Body.Bytes(), &zone)
				require.NoError(t, err)
				assert.Equal(t, tt.input.(models.UpdateZoneRequest).Name, zone.Name)
				assert.Equal(t, tt.input.(models.UpdateZoneRequest).Timezone, zone.Timezone)
			}
		})
	}
}

func TestZoneHandler_DeleteZone(t *testing.T) {
	tests := []zoneTest{
		{
			name: "Valid Delete (Admin)",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				_, err := tc.DB.Exec(`INSERT INTO zones (id, name, timezone) VALUES ($1, 'TEST1', 'Europe/London')`, uuid.New())
				require.NoError(t, err)
				return tc.GetTestJWT(admin.ID)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "Not Authorized (Regular User)",
			setupFunc: func(tc *testutil.TestContext) string {
				user := tc.CreateTestUser("user", "user@test.com", "password123", false)
				_, err := tc.DB.Exec(`INSERT INTO zones (id, name, timezone) VALUES ($1, 'TEST1', 'Europe/London')`, uuid.New())
				require.NoError(t, err)
				return tc.GetTestJWT(user.ID)
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
		},
		{
			name: "Not Authenticated",
			setupFunc: func(tc *testutil.TestContext) string {
				_, err := tc.DB.Exec(`INSERT INTO zones (id, name, timezone) VALUES ($1, 'TEST1', 'Europe/London')`, uuid.New())
				require.NoError(t, err)
				return ""
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name: "Non-existent Zone",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			wantStatus: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name: "Zone Has Spot Prices",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				zoneID := uuid.New()
				_, err := tc.DB.Exec(`INSERT INTO zones (id, name, timezone) VALUES ($1, 'TEST1', 'Europe/London')`, zoneID)
				require.NoError(t, err)

				// Get a currency ID to use
				var currencyID uuid.UUID
				err = tc.DB.QueryRow(`SELECT id FROM currencies WHERE name = 'EUR'`).Scan(&currencyID)
				require.NoError(t, err)

				// Create a spot price using this zone
				_, err = tc.DB.Exec(`
					INSERT INTO spot_prices (id, timestamp, zone_id, currency_id, price)
					VALUES ($1, NOW(), $2, $3, 42.50)
				`, uuid.New(), zoneID, currencyID)
				require.NoError(t, err)

				return tc.GetTestJWT(admin.ID)
			},
			wantStatus: http.StatusConflict,
			wantErr:    true,
			errMsg:     "cannot delete zone that has associated spot prices",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			token := ""
			var zoneID uuid.UUID
			if tt.setupFunc != nil {
				token = tt.setupFunc(tc)
				if tt.name != "Non-existent Zone" {
					err := tc.DB.QueryRow(`SELECT id FROM zones WHERE name = 'TEST1'`).Scan(&zoneID)
					require.NoError(t, err)
				} else {
					zoneID = uuid.New()
				}
			}

			handler := handlers.NewZoneHandler(postgres.NewZoneRepository(tc.DB))
			router := gin.New()
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router.Use(authMiddleware.AuthRequired())
			router.DELETE("/zones/:id", authMiddleware.AdminRequired(), handler.DeleteZone)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("DELETE", "/zones/"+zoneID.String(), nil)
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.errMsg != "" {
				var errResp models.ErrorResponse
				err := json.NewDecoder(w.Body).Decode(&errResp)
				require.NoError(t, err)
				assert.Equal(t, tt.errMsg, errResp.Error)
			}

			if !tt.wantErr && tt.wantStatus == http.StatusNoContent {
				// Verify zone was deleted
				var count int
				err := tc.DB.QueryRow(`SELECT COUNT(*) FROM zones WHERE id = $1`, zoneID).Scan(&count)
				require.NoError(t, err)
				assert.Equal(t, 0, count)
			}
		})
	}
}
