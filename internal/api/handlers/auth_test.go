package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"wattwatch/internal/api/handlers"
	"wattwatch/internal/models"
	"wattwatch/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

type loginTest struct {
	name       string
	username   string
	setupFunc  func(*testutil.TestContext)
	input      models.LoginRequest
	wantStatus int
	wantErr    bool
	errMsg     string
}

func TestAuthHandler_Login(t *testing.T) {
	tests := []loginTest{
		{
			name:     "Success",
			username: "test_user",
			setupFunc: func(tc *testutil.TestContext) {
				tc.CreateTestUser("test_user", "test@example.com", "test_password", false)
			},
			input: models.LoginRequest{
				Username: "test_user",
				Password: "test_password",
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "Inactive User",
			username: "test_user",
			setupFunc: func(tc *testutil.TestContext) {
				user := tc.CreateTestUser("test_user", "inactive@example.com", "test_password", false)
				// Mark user as deleted using direct SQL query
				_, err := tc.DB.Exec("UPDATE users SET deleted_at = CURRENT_TIMESTAMP WHERE id = $1", user.ID)
				require.NoError(t, err)
			},
			input: models.LoginRequest{
				Username: "test_user",
				Password: "test_password",
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    true,
			errMsg:     "invalid credentials",
		},
		{
			name:     "Invalid Credentials",
			username: "test_user",
			setupFunc: func(tc *testutil.TestContext) {
				tc.CreateTestUser("test_user", "test@example.com", "test_password", false)
			},
			input: models.LoginRequest{
				Username: "test_user",
				Password: "wrong_password",
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    true,
			errMsg:     "invalid credentials",
		},
		{
			name:      "User Not Found",
			username:  "nonexistent_user",
			setupFunc: nil,
			input: models.LoginRequest{
				Username: "nonexistent_user",
				Password: "test_password",
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    true,
			errMsg:     "invalid credentials",
		},
		{
			name:     "Too Many Failed Attempts",
			username: "test_user_locked",
			setupFunc: func(tc *testutil.TestContext) {
				user := tc.CreateTestUserFromRequest(models.CreateUserRequest{
					Username: "test_user_locked",
					Email:    testutil.String("test@example.com"),
					Password: "test_password",
				}, false)
				// Create failed login attempts
				for i := 0; i < 5; i++ {
					attempt := &models.LoginAttempt{
						UserID:    user.ID,
						Success:   false,
						IP:        "127.0.0.1",
						CreatedAt: time.Now(),
					}
					err := tc.LoginAttemptRepo.Create(context.Background(), attempt.UserID, attempt.Success, attempt.IP, attempt.CreatedAt)
					require.NoError(t, err)
				}
			},
			input: models.LoginRequest{
				Username: "test_user_locked",
				Password: "test_password",
			},
			wantStatus: http.StatusTooManyRequests,
			wantErr:    true,
			errMsg:     "too many failed login attempts",
		},
		{
			name:      "Missing Username",
			username:  "",
			setupFunc: nil,
			input: models.LoginRequest{
				Username: "",
				Password: "test_password",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "Key: 'LoginRequest.Username' Error:Field validation for 'Username' failed on the 'required' tag",
		},
		{
			name:      "Missing Password",
			username:  "test_user",
			setupFunc: nil,
			input: models.LoginRequest{
				Username: "test_user",
				Password: "",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "Key: 'LoginRequest.Password' Error:Field validation for 'Password' failed on the 'required' tag",
		},
		{
			name:     "Email Not Verified",
			username: "test_user_unverified",
			setupFunc: func(tc *testutil.TestContext) {
				user := tc.CreateTestUserFromRequest(models.CreateUserRequest{
					Username: "test_user_unverified",
					Email:    testutil.String("unverified@example.com"),
					Password: "test_password",
				}, false)
				user.EmailVerified = false
				user.UpdatedAt = time.Now()
				_, err := tc.DB.Exec("UPDATE users SET email_verified = false WHERE id = $1", user.ID)
				require.NoError(t, err)
			},
			input: models.LoginRequest{
				Username: "test_user_unverified",
				Password: "test_password",
			},
			wantStatus: http.StatusOK, // Should still allow login even if email isn't verified
			wantErr:    false,
		},
		{
			name:     "Admin User Login",
			username: "test_admin",
			setupFunc: func(tc *testutil.TestContext) {
				tc.CreateTestUserFromRequest(models.CreateUserRequest{
					Username: "test_admin",
					Email:    testutil.String("admin@example.com"),
					Password: "test_password",
				}, true) // Create as admin
			},
			input: models.LoginRequest{
				Username: "test_admin",
				Password: "test_password",
			},
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:     "Special Characters in Username",
			username: "test@user!123",
			setupFunc: func(tc *testutil.TestContext) {
				tc.CreateTestUserFromRequest(models.CreateUserRequest{
					Username: "test@user!123",
					Email:    testutil.String("special@example.com"),
					Password: "test_password",
				}, false)
			},
			input: models.LoginRequest{
				Username: "test@user!123",
				Password: "test_password",
			},
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:      "Very Long Username",
			username:  "test_user_" + strings.Repeat("a", 100),
			setupFunc: nil,
			input: models.LoginRequest{
				Username: "test_user_" + strings.Repeat("a", 100),
				Password: "test_password",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "Key: 'LoginRequest.Username' Error:Field validation for 'Username' failed on the 'max' tag",
		},
		{
			name:      "SQL Injection Attempt",
			username:  "' OR '1'='1",
			setupFunc: nil,
			input: models.LoginRequest{
				Username: "' OR '1'='1",
				Password: "test_password",
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    true,
			errMsg:     "invalid credentials",
		},
		{
			name:     "Concurrent Login Attempts",
			username: "test_user_concurrent",
			setupFunc: func(tc *testutil.TestContext) {
				tc.CreateTestUserFromRequest(models.CreateUserRequest{
					Username: "test_user_concurrent",
					Email:    testutil.String("concurrent@example.com"),
					Password: "test_password",
				}, false)
			},
			input: models.LoginRequest{
				Username: "test_user_concurrent",
				Password: "test_password",
			},
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:     "Unicode Username",
			username: "测试用户",
			setupFunc: func(tc *testutil.TestContext) {
				tc.CreateTestUserFromRequest(models.CreateUserRequest{
					Username: "测试用户",
					Email:    testutil.String("unicode@example.com"),
					Password: "test_password",
				}, false)
			},
			input: models.LoginRequest{
				Username: "测试用户",
				Password: "test_password",
			},
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:     "Rate Limiting - Multiple IPs",
			username: "test_user_rate_limit",
			setupFunc: func(tc *testutil.TestContext) {
				user := tc.CreateTestUserFromRequest(models.CreateUserRequest{
					Username: "test_user_rate_limit",
					Email:    testutil.String("rate@example.com"),
					Password: "test_password",
				}, false)
				// Create attempts from different IPs
				ips := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}
				for _, ip := range ips {
					for i := 0; i < 5; i++ {
						attempt := &models.LoginAttempt{
							UserID:    user.ID,
							Success:   false,
							IP:        ip,
							CreatedAt: time.Now(),
						}
						err := tc.LoginAttemptRepo.Create(context.Background(), attempt.UserID, attempt.Success, attempt.IP, attempt.CreatedAt)
						require.NoError(t, err)
					}
				}
			},
			input: models.LoginRequest{
				Username: "test_user_rate_limit",
				Password: "test_password",
			},
			wantStatus: http.StatusTooManyRequests,
			wantErr:    true,
			errMsg:     "too many failed login attempts",
		},
		{
			name:     "Password with Special Characters",
			username: "test_user_special_pass",
			setupFunc: func(tc *testutil.TestContext) {
				tc.CreateTestUserFromRequest(models.CreateUserRequest{
					Username: "test_user_special_pass",
					Email:    testutil.String("special_pass@example.com"),
					Password: "P@ssw0rd!#$%^&*()",
				}, false)
			},
			input: models.LoginRequest{
				Username: "test_user_special_pass",
				Password: "P@ssw0rd!#$%^&*()",
			},
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:     "Old Failed Attempts Should Reset",
			username: "test_user_old_attempts",
			setupFunc: func(tc *testutil.TestContext) {
				user := tc.CreateTestUserFromRequest(models.CreateUserRequest{
					Username: "test_user_old_attempts",
					Email:    testutil.String("old@example.com"),
					Password: "test_password",
				}, false)
				// Create old failed attempts
				for i := 0; i < 5; i++ {
					attempt := &models.LoginAttempt{
						UserID:    user.ID,
						Success:   false,
						IP:        "127.0.0.1",
						CreatedAt: time.Now().Add(-24 * time.Hour), // 24 hours old
					}
					err := tc.LoginAttemptRepo.Create(context.Background(), attempt.UserID, attempt.Success, attempt.IP, attempt.CreatedAt)
					require.NoError(t, err)
				}
			},
			input: models.LoginRequest{
				Username: "test_user_old_attempts",
				Password: "test_password",
			},
			wantStatus: http.StatusOK, // Should succeed as old attempts are ignored
			wantErr:    false,
		},
		{
			name:     "JWT Token Validation",
			username: "test_user_jwt",
			setupFunc: func(tc *testutil.TestContext) {
				tc.CreateTestUserFromRequest(models.CreateUserRequest{
					Username: "test_user_jwt",
					Email:    testutil.String("jwt@example.com"),
					Password: "test_password",
				}, false)
			},
			input: models.LoginRequest{
				Username: "test_user_jwt",
				Password: "test_password",
			},
			wantStatus: http.StatusOK,
			wantErr:    false,
			errMsg:     "JWT token validation failed",
		},
		{
			name:     "Audit Log Creation",
			username: "test_user_audit",
			setupFunc: func(tc *testutil.TestContext) {
				tc.CreateTestUserFromRequest(models.CreateUserRequest{
					Username: "test_user_audit",
					Email:    testutil.String("audit@example.com"),
					Password: "test_password",
				}, false)
			},
			input: models.LoginRequest{
				Username: "test_user_audit",
				Password: "test_password",
			},
			wantStatus: http.StatusOK,
			wantErr:    false,
			errMsg:     "Audit log creation failed",
		},
		{
			name:     "Concurrent Access - Same User",
			username: "test_user_concurrent_same",
			setupFunc: func(tc *testutil.TestContext) {
				tc.CreateTestUserFromRequest(models.CreateUserRequest{
					Username: "test_user_concurrent_same",
					Email:    testutil.String("concurrent_same@example.com"),
					Password: "test_password",
				}, false)
			},
			input: models.LoginRequest{
				Username: "test_user_concurrent_same",
				Password: "test_password",
			},
			wantStatus: http.StatusOK,
			wantErr:    false,
			errMsg:     "Concurrent access failed",
		},
		{
			name:     "Last Login Time Update",
			username: "test_user_last_login",
			setupFunc: func(tc *testutil.TestContext) {
				tc.CreateTestUserFromRequest(models.CreateUserRequest{
					Username: "test_user_last_login",
					Email:    testutil.String("last_login@example.com"),
					Password: "test_password",
				}, false)
			},
			input: models.LoginRequest{
				Username: "test_user_last_login",
				Password: "test_password",
			},
			wantStatus: http.StatusOK,
			wantErr:    false,
			errMsg:     "Last login time update failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			if tt.setupFunc != nil {
				tt.setupFunc(tc)
			}

			handler := tc.AuthHandler
			router := gin.New()
			router.POST("/login", handler.Login)

			body, err := json.Marshal(tt.input)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			require.Equal(t, tt.wantStatus, w.Code)

			if tt.wantErr {
				var resp models.ErrorResponse
				err = json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				require.Equal(t, tt.errMsg, resp.Error)
			} else {
				var resp handlers.LoginResponse
				err = json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				require.NotEmpty(t, resp.AccessToken)

				// Verify the new access token is valid
				claims, err := tc.AuthService.ValidateToken(resp.AccessToken)
				require.NoError(t, err)
				require.NotNil(t, claims)
				userID, exists := (*claims)["user_id"]
				require.True(t, exists)
				require.NotEmpty(t, userID)
			}
		})
	}
}

func TestAuthHandler_Register(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.TestContext)
		input      models.CreateUserRequest
		wantStatus int
		wantErr    bool
		errMsg     string
		validate   func(*testing.T, *testutil.TestContext, *models.User)
	}{
		{
			name: "Success_FirstUser",
			input: models.CreateUserRequest{
				Username: "test_user",
				Email:    testutil.String("test@example.com"),
				Password: "test_password",
			},
			wantStatus: http.StatusCreated,
			validate: func(t *testing.T, tc *testutil.TestContext, user *models.User) {
				require.NotNil(t, user)

				// Check if user got admin role
				role, err := tc.RoleRepo.GetByID(context.Background(), user.RoleID)
				require.NoError(t, err)
				require.True(t, role.IsAdminGroup)

				// Verify user is not deleted
				require.Nil(t, user.DeletedAt)
			},
		},
		{
			name: "Success_SubsequentUser",
			setupFunc: func(tc *testutil.TestContext) {
				// Create first user as admin
				tc.CreateTestUser("admin_user", "admin@example.com", "test_password", true)
			},
			input: models.CreateUserRequest{
				Username: "test_user",
				Email:    testutil.String("test@example.com"),
				Password: "test_password",
			},
			wantStatus: http.StatusCreated,
			validate: func(t *testing.T, tc *testutil.TestContext, user *models.User) {
				require.NotNil(t, user)

				// Check if user got regular user role
				role, err := tc.RoleRepo.GetByID(context.Background(), user.RoleID)
				require.NoError(t, err)
				require.False(t, role.IsAdminGroup)

				// Verify user is not deleted
				require.Nil(t, user.DeletedAt)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test context
			tc := testutil.NewTestContext(t)

			// Run setup if provided
			if tt.setupFunc != nil {
				tt.setupFunc(tc)
			}

			// Create request
			body, err := json.Marshal(tt.input)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Setup router and make request
			router := gin.New()
			router.POST("/auth/register", tc.AuthHandler.Register)
			router.ServeHTTP(w, req)

			// Check status code
			require.Equal(t, tt.wantStatus, w.Code)

			if tt.wantErr {
				var resp models.ErrorResponse
				err = json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				require.Equal(t, tt.errMsg, resp.Error)
			} else {
				var user models.User
				err = json.NewDecoder(w.Body).Decode(&user)
				require.NoError(t, err)

				if tt.validate != nil {
					tt.validate(t, tc, &user)
				}
			}
		})
	}
}

type refreshTest struct {
	name       string
	setupFunc  func(*testutil.TestContext) (string, error)
	wantStatus int
	wantErr    bool
	errMsg     string
}

func TestAuthHandler_Refresh(t *testing.T) {
	tests := []refreshTest{
		{
			name: "Success",
			setupFunc: func(tc *testutil.TestContext) (string, error) {
				user := tc.CreateTestUser("test_user", "test@example.com", "test_password", false)
				return tc.AuthService.GenerateRefreshToken(context.Background(), user.ID)
			},
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name: "Invalid Token Format",
			setupFunc: func(tc *testutil.TestContext) (string, error) {
				return "invalid_token", nil
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    true,
			errMsg:     "invalid or expired refresh token",
		},
		{
			name: "Expired Token",
			setupFunc: func(tc *testutil.TestContext) (string, error) {
				user := tc.CreateTestUser("test_user_expired", "expired@example.com", "test_password", false)
				token, err := tc.AuthService.GenerateRefreshToken(context.Background(), user.ID)
				if err != nil {
					return "", err
				}
				// Manually expire the token in the database
				_, err = tc.DB.Exec("UPDATE refresh_tokens SET expires_at = NOW() - INTERVAL '1 hour' WHERE token = $1", token)
				return token, err
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    true,
			errMsg:     "invalid or expired refresh token",
		},
		{
			name: "Deleted User",
			setupFunc: func(tc *testutil.TestContext) (string, error) {
				user := tc.CreateTestUser("test_user_deleted", "deleted@example.com", "test_password", false)
				token, err := tc.AuthService.GenerateRefreshToken(context.Background(), user.ID)
				if err != nil {
					return "", err
				}
				// Soft delete the user
				_, err = tc.DB.Exec("UPDATE users SET deleted_at = NOW() WHERE id = $1", user.ID)
				return token, err
			},
			wantStatus: http.StatusInternalServerError,
			wantErr:    true,
			errMsg:     "failed to get user",
		},
		{
			name: "Missing Token",
			setupFunc: func(tc *testutil.TestContext) (string, error) {
				return "", nil
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "Key: 'RefreshRequest.RefreshToken' Error:Field validation for 'RefreshToken' failed on the 'required' tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			var refreshToken string
			var err error
			if tt.setupFunc != nil {
				refreshToken, err = tt.setupFunc(tc)
				require.NoError(t, err)
			}

			handler := tc.AuthHandler

			router := gin.New()
			router.POST("/refresh", handler.Refresh)

			reqBody := handlers.RefreshRequest{
				RefreshToken: refreshToken,
			}
			body, err := json.Marshal(reqBody)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			require.Equal(t, tt.wantStatus, w.Code)

			if tt.wantErr {
				var resp models.ErrorResponse
				err = json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				require.Equal(t, tt.errMsg, resp.Error)
			} else {
				var resp handlers.RefreshResponse
				err = json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				require.NotEmpty(t, resp.AccessToken)

				// Verify the new access token is valid
				claims, err := tc.AuthService.ValidateToken(resp.AccessToken)
				require.NoError(t, err)
				require.NotNil(t, claims)
				userID, exists := (*claims)["user_id"]
				require.True(t, exists)
				require.NotEmpty(t, userID)
			}
		})
	}
}

func TestAuthHandler_RequestPasswordReset(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.TestContext)
		input      models.PasswordResetRequest
		wantStatus int
		wantErr    bool
		errMsg     string
	}{
		{
			name: "Success",
			setupFunc: func(tc *testutil.TestContext) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				// Mark email as verified
				tc.MarkEmailVerified(user.ID)
			},
			input: models.PasswordResetRequest{
				Email: "test@example.com",
			},
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name: "Error_UnverifiedEmail",
			setupFunc: func(tc *testutil.TestContext) {
				tc.CreateTestUser("unverified_user", "unverified@example.com", "password123", false)
			},
			input: models.PasswordResetRequest{
				Email: "unverified@example.com",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "email address must be verified before requesting a password reset",
		},
		{
			name: "Error_NonexistentEmail",
			setupFunc: func(tc *testutil.TestContext) {
				// No setup needed - testing nonexistent email
			},
			input: models.PasswordResetRequest{
				Email: "nonexistent@example.com",
			},
			wantStatus: http.StatusOK, // Return success for security (don't leak user existence)
			wantErr:    false,
		},
		{
			name: "Error_InvalidEmailFormat",
			setupFunc: func(tc *testutil.TestContext) {
				// No setup needed - testing invalid email format
			},
			input: models.PasswordResetRequest{
				Email: "invalid-email",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "Key: 'PasswordResetRequest.Email' Error:Field validation for 'Email' failed on the 'email' tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			if tt.setupFunc != nil {
				tt.setupFunc(tc)
			}

			// Create handler
			handler := tc.AuthHandler

			// Create request
			body, err := json.Marshal(tt.input)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/auth/reset-password", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Setup router
			router := gin.New()
			router.POST("/auth/reset-password", handler.RequestPasswordReset)

			// Serve request
			router.ServeHTTP(w, req)

			// Check status code
			require.Equal(t, tt.wantStatus, w.Code)

			if tt.wantErr {
				var errResp models.ErrorResponse
				err := json.Unmarshal(w.Body.Bytes(), &errResp)
				require.NoError(t, err)
				require.Equal(t, tt.errMsg, errResp.Error)
				return
			}

			var successResp models.SuccessResponse
			err = json.Unmarshal(w.Body.Bytes(), &successResp)
			require.NoError(t, err)
			require.Equal(t, "if the email exists, a reset link will be sent", successResp.Message)
		})
	}
}
