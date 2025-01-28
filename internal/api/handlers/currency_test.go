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

type currencyTest struct {
	name       string
	setupFunc  func(*testutil.TestContext) string
	input      interface{}
	wantStatus int
	wantErr    bool
	errMsg     string
}

func TestCurrencyHandler_ListCurrencies(t *testing.T) {
	tc := testutil.NewTestContext(t)

	handler := handlers.NewCurrencyHandler(postgres.NewCurrencyRepository(tc.DB))
	router := gin.New()
	router.GET("/currencies", handler.ListCurrencies)
	router.GET("/currencies/:id", handler.GetCurrency)

	// Insert test data (using currencies not in defaults)
	currency1ID := uuid.New()
	currency2ID := uuid.New()
	_, err := tc.DB.Exec(`
		INSERT INTO currencies (id, name) VALUES
		($1, 'USD'),
		($2, 'GBP')
	`, currency1ID, currency2ID)
	require.NoError(t, err)

	// Test listing currencies (should include defaults + inserted)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/currencies", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var currencies []models.Currency
	err = json.Unmarshal(w.Body.Bytes(), &currencies)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(currencies), 4) // At least 2 defaults + 2 inserted

	// Test getting a specific currency
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/currencies/"+currency1ID.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var currency models.Currency
	err = json.Unmarshal(w.Body.Bytes(), &currency)
	require.NoError(t, err)
	assert.Equal(t, currency1ID, currency.ID)
	assert.Equal(t, "USD", currency.Name)

	// Test getting non-existent currency
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/currencies/"+uuid.New().String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCurrencyHandler_GetCurrency(t *testing.T) {
	tc := testutil.NewTestContext(t)

	handler := handlers.NewCurrencyHandler(postgres.NewCurrencyRepository(tc.DB))
	router := gin.New()
	router.GET("/currencies/:id", handler.GetCurrency)

	// Insert test data
	currencyID := uuid.New()
	_, err := tc.DB.Exec(`
		INSERT INTO currencies (id, name) VALUES ($1, 'USD')
	`, currencyID)
	require.NoError(t, err)

	// Test getting existing currency
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/currencies/"+currencyID.String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var currency models.Currency
	err = json.Unmarshal(w.Body.Bytes(), &currency)
	require.NoError(t, err)
	assert.Equal(t, currencyID, currency.ID)
	assert.Equal(t, "USD", currency.Name)

	// Test getting non-existent currency
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/currencies/"+uuid.New().String(), nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCurrencyHandler_CreateCurrency(t *testing.T) {
	tests := []currencyTest{
		{
			name: "Valid Currency (Admin)",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateCurrencyRequest{
				Name: "JPY",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "Not Authorized (Regular User)",
			setupFunc: func(tc *testutil.TestContext) string {
				user := tc.CreateTestUser("user", "user@test.com", "password123", false)
				return tc.GetTestJWT(user.ID)
			},
			input: models.CreateCurrencyRequest{
				Name: "JPY",
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
		},
		{
			name: "Not Authenticated",
			setupFunc: func(tc *testutil.TestContext) string {
				return ""
			},
			input: models.CreateCurrencyRequest{
				Name: "JPY",
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name: "Invalid Currency (2 letters)",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateCurrencyRequest{
				Name: "JP",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "Invalid Currency (4 letters)",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateCurrencyRequest{
				Name: "JPYN",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "Duplicate Currency",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateCurrencyRequest{
				Name: "EUR", // Try to create existing default currency
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

			handler := handlers.NewCurrencyHandler(postgres.NewCurrencyRepository(tc.DB))
			router := gin.New()
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router.Use(authMiddleware.AuthRequired())
			router.POST("/currencies", authMiddleware.AdminRequired(), handler.CreateCurrency)

			body, err := json.Marshal(tt.input)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/currencies", bytes.NewBuffer(body))
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if !tt.wantErr {
				var currency models.Currency
				err = json.Unmarshal(w.Body.Bytes(), &currency)
				require.NoError(t, err)
				assert.Equal(t, tt.input.(models.CreateCurrencyRequest).Name, currency.Name)
			}
		})
	}
}

func TestCurrencyHandler_UpdateCurrency(t *testing.T) {
	tests := []currencyTest{
		{
			name: "Valid Update (Admin)",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				_, err := tc.DB.Exec(`INSERT INTO currencies (id, name) VALUES ($1, 'USD')`, uuid.New())
				require.NoError(t, err)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateCurrencyRequest{
				Name: "GBP",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Not Authorized (Regular User)",
			setupFunc: func(tc *testutil.TestContext) string {
				user := tc.CreateTestUser("user", "user@test.com", "password123", false)
				_, err := tc.DB.Exec(`INSERT INTO currencies (id, name) VALUES ($1, 'USD')`, uuid.New())
				require.NoError(t, err)
				return tc.GetTestJWT(user.ID)
			},
			input: models.UpdateCurrencyRequest{
				Name: "GBP",
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
		},
		{
			name: "Not Authenticated",
			setupFunc: func(tc *testutil.TestContext) string {
				_, err := tc.DB.Exec(`INSERT INTO currencies (id, name) VALUES ($1, 'USD')`, uuid.New())
				require.NoError(t, err)
				return ""
			},
			input: models.UpdateCurrencyRequest{
				Name: "GBP",
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name: "Invalid Update (2 letters)",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				_, err := tc.DB.Exec(`INSERT INTO currencies (id, name) VALUES ($1, 'USD')`, uuid.New())
				require.NoError(t, err)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateCurrencyRequest{
				Name: "GB",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name: "Update to Existing Name",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				_, err := tc.DB.Exec(`INSERT INTO currencies (id, name) VALUES ($1, 'USD')`, uuid.New())
				require.NoError(t, err)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateCurrencyRequest{
				Name: "EUR", // Try to update to existing default currency
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

			handler := handlers.NewCurrencyHandler(postgres.NewCurrencyRepository(tc.DB))
			router := gin.New()
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router.Use(authMiddleware.AuthRequired())
			router.PUT("/currencies/:id", authMiddleware.AdminRequired(), handler.UpdateCurrency)

			// Get an existing currency ID
			var currencyID uuid.UUID
			err := tc.DB.QueryRow(`SELECT id FROM currencies WHERE name = 'USD'`).Scan(&currencyID)
			require.NoError(t, err)

			body, err := json.Marshal(tt.input)
			require.NoError(t, err)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("PUT", "/currencies/"+currencyID.String(), bytes.NewBuffer(body))
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if !tt.wantErr {
				var currency models.Currency
				err = json.Unmarshal(w.Body.Bytes(), &currency)
				require.NoError(t, err)
				assert.Equal(t, tt.input.(models.UpdateCurrencyRequest).Name, currency.Name)
			}
		})
	}
}

func TestCurrencyHandler_DeleteCurrency(t *testing.T) {
	tests := []currencyTest{
		{
			name: "Valid Delete (Admin)",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				_, err := tc.DB.Exec(`INSERT INTO currencies (id, name) VALUES ($1, 'USD')`, uuid.New())
				require.NoError(t, err)
				return tc.GetTestJWT(admin.ID)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "Not Authorized (Regular User)",
			setupFunc: func(tc *testutil.TestContext) string {
				user := tc.CreateTestUser("user", "user@test.com", "password123", false)
				_, err := tc.DB.Exec(`INSERT INTO currencies (id, name) VALUES ($1, 'USD')`, uuid.New())
				require.NoError(t, err)
				return tc.GetTestJWT(user.ID)
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
		},
		{
			name: "Not Authenticated",
			setupFunc: func(tc *testutil.TestContext) string {
				_, err := tc.DB.Exec(`INSERT INTO currencies (id, name) VALUES ($1, 'USD')`, uuid.New())
				require.NoError(t, err)
				return ""
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name: "Non-existent Currency",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			wantStatus: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name: "Currency Has Spot Prices",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				currencyID := uuid.New()
				_, err := tc.DB.Exec(`INSERT INTO currencies (id, name) VALUES ($1, 'USD')`, currencyID)
				require.NoError(t, err)

				// Get a zone ID to use
				var zoneID uuid.UUID
				err = tc.DB.QueryRow(`SELECT id FROM zones WHERE name = 'SE1'`).Scan(&zoneID)
				require.NoError(t, err)

				// Create a spot price using this currency
				_, err = tc.DB.Exec(`
					INSERT INTO spot_prices (id, timestamp, zone_id, currency_id, price)
					VALUES ($1, NOW(), $2, $3, 42.50)
				`, uuid.New(), zoneID, currencyID)
				require.NoError(t, err)

				return tc.GetTestJWT(admin.ID)
			},
			wantStatus: http.StatusConflict,
			wantErr:    true,
			errMsg:     "cannot delete currency that has associated spot prices",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			token := ""
			var currencyID uuid.UUID
			if tt.setupFunc != nil {
				token = tt.setupFunc(tc)
				if tt.name != "Non-existent Currency" {
					err := tc.DB.QueryRow(`SELECT id FROM currencies WHERE name = 'USD'`).Scan(&currencyID)
					require.NoError(t, err)
				} else {
					currencyID = uuid.New()
				}
			}

			handler := handlers.NewCurrencyHandler(postgres.NewCurrencyRepository(tc.DB))
			router := gin.New()
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router.Use(authMiddleware.AuthRequired())
			router.DELETE("/currencies/:id", authMiddleware.AdminRequired(), handler.DeleteCurrency)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("DELETE", "/currencies/"+currencyID.String(), nil)
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
				// Verify currency was deleted
				var count int
				err := tc.DB.QueryRow(`SELECT COUNT(*) FROM currencies WHERE id = $1`, currencyID).Scan(&count)
				require.NoError(t, err)
				assert.Equal(t, 0, count)
			}
		})
	}
}
