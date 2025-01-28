package handlers_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"wattwatch/internal/api/handlers"
	"wattwatch/internal/models"
	"wattwatch/internal/testutil"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler_Health(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.TestContext) *sql.DB
		wantStatus int
		wantErr    bool
	}{
		{
			name: "Success",
			setupFunc: func(tc *testutil.TestContext) *sql.DB {
				return tc.DB // Use the test database which should be healthy
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Error_DatabaseDown",
			setupFunc: func(tc *testutil.TestContext) *sql.DB {
				// Create a new connection with invalid credentials to simulate failure
				connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
					tc.Config.Database.Host,
					tc.Config.Database.Port,
					"invalid_user",
					"invalid_password",
					tc.Config.Database.DBName,
					tc.Config.Database.SSLMode,
				)
				db, err := sql.Open("postgres", connStr)
				require.NoError(t, err)
				return db
			},
			wantStatus: http.StatusServiceUnavailable,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			// Setup test case and get DB to use
			db := tt.setupFunc(tc)

			// Create handler with the test-specific DB
			handler := handlers.NewHealthHandler(db)

			// Setup router
			router := gin.New()
			router.GET("/health", handler.Health)

			// Make request
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/health", nil)
			router.ServeHTTP(w, req)

			// Check status code
			require.Equal(t, tt.wantStatus, w.Code)

			if tt.wantErr {
				var errResp models.ErrorResponse
				err := json.Unmarshal(w.Body.Bytes(), &errResp)
				require.NoError(t, err)
				require.Equal(t, "database connection failed", errResp.Error)
			} else {
				var resp models.HealthResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, "healthy", resp.Status)
			}

			// Close the DB if it's not the test context's DB
			if db != tc.DB {
				db.Close()
			}
		})
	}
}
