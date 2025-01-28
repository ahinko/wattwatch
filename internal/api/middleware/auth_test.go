package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"wattwatch/internal/api/middleware"
	"wattwatch/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestAuthMiddleware_AuthRequired(t *testing.T) {
	tests := []struct {
		name       string
		setupAuth  func(*testutil.TestContext) string
		wantStatus int
		wantErr    string
	}{
		{
			name: "Valid Token",
			setupAuth: func(tc *testutil.TestContext) string {
				user := tc.CreateTestUser("testuser", "test@example.com", "password123", false)
				return tc.GetTestJWT(user.ID)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Missing Authorization Header",
			setupAuth: func(tc *testutil.TestContext) string {
				return ""
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    "no authorization header",
		},
		{
			name: "Invalid Authorization Header Format",
			setupAuth: func(tc *testutil.TestContext) string {
				return "InvalidFormat Token"
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    "invalid authorization header",
		},
		{
			name: "Invalid Token",
			setupAuth: func(tc *testutil.TestContext) string {
				// Create a token signed with a different secret
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"user_id":  "invalid-uuid",
					"username": "test",
					"is_admin": false,
					"exp":      time.Now().Add(time.Hour).Unix(),
				})
				tokenString, err := token.SignedString([]byte("wrong-secret"))
				require.NoError(t, err)
				return tokenString
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    "invalid token",
		},
		{
			name: "User Not Found",
			setupAuth: func(tc *testutil.TestContext) string {
				// Create user, get token, then delete user
				user := tc.CreateTestUser("deleteduser", "deleted@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				err := tc.UserRepo.Delete(context.Background(), user.ID)
				require.NoError(t, err)
				return token
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    "user not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			// Create middleware
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)

			// Setup test handler
			router := gin.New()
			router.GET("/test", authMiddleware.AuthRequired(), func(c *gin.Context) {
				// For successful auth, verify user is in context
				user, exists := c.Get("user")
				require.True(t, exists)
				require.NotNil(t, user)
				c.Status(http.StatusOK)
			})

			// Make request
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test", nil)
			if token := tt.setupAuth(tc); token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			router.ServeHTTP(w, req)

			// Check response
			require.Equal(t, tt.wantStatus, w.Code)
			if tt.wantErr != "" {
				var resp gin.H
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				require.Equal(t, tt.wantErr, resp["error"])
			}
		})
	}
}

func TestAuthMiddleware_AdminRequired(t *testing.T) {
	tests := []struct {
		name       string
		setupAuth  func(*testutil.TestContext) string
		wantStatus int
		wantErr    string
	}{
		{
			name: "Admin Access Allowed",
			setupAuth: func(tc *testutil.TestContext) string {
				user := tc.CreateTestUser("adminuser", "admin@example.com", "password123", true)
				return tc.GetTestJWT(user.ID)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Non-Admin Access Denied",
			setupAuth: func(tc *testutil.TestContext) string {
				user := tc.CreateTestUser("regularuser", "user@example.com", "password123", false)
				return tc.GetTestJWT(user.ID)
			},
			wantStatus: http.StatusForbidden,
			wantErr:    "admin access required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			// Create middleware
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)

			// Setup test handler
			router := gin.New()
			router.GET("/test",
				authMiddleware.AuthRequired(),  // First authenticate
				authMiddleware.AdminRequired(), // Then check admin
				func(c *gin.Context) {
					c.Status(http.StatusOK)
				},
			)

			// Make request
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test", nil)
			if token := tt.setupAuth(tc); token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			router.ServeHTTP(w, req)

			// Check response
			require.Equal(t, tt.wantStatus, w.Code)
			if tt.wantErr != "" {
				var resp gin.H
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				require.Equal(t, tt.wantErr, resp["error"])
			}
		})
	}
}
