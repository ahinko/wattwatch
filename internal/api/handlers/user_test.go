package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"wattwatch/internal/api/handlers"
	"wattwatch/internal/api/middleware"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"
	"wattwatch/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type updateUserTest struct {
	name        string
	setupFunc   func(*testutil.TestContext) (uuid.UUID, string)
	input       models.UpdateUserRequest
	invalidJSON bool
	wantStatus  int
	wantErr     bool
	errMsg      string
}

func TestUserHandler_UpdateUser(t *testing.T) {
	tests := []updateUserTest{
		{
			name: "Success_UpdateEmail",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return user.ID, token
			},
			input: models.UpdateUserRequest{
				Email: testutil.String("new@example.com"),
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Error_UserNotFound",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return uuid.New(), token // Return a random UUID that doesn't exist in the database
			},
			input: models.UpdateUserRequest{
				Email: testutil.String("new@example.com"),
			},
			wantStatus: http.StatusNotFound,
			wantErr:    true,
			errMsg:     "user not found",
		},
		{
			name: "Error_InvalidEmail",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return user.ID, token
			},
			input: models.UpdateUserRequest{
				Email: testutil.String("invalid-email"),
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "invalid email address",
		},
		{
			name: "Error_NonAdminUpdatingOtherUser",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				// Create target user
				targetUser := tc.CreateTestUser("target_user", "target@example.com", "test_password", false)

				// Create another non-admin user and get their token
				otherUser := tc.CreateTestUser("other_user", "other@example.com", "test_password", false)
				token := tc.GetTestJWT(otherUser.ID)

				return targetUser.ID, token
			},
			input: models.UpdateUserRequest{
				Email: testutil.String("new@example.com"),
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
			errMsg:     "permission denied",
		},
		{
			name: "Error_NonAdminUpdatingRole",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "test_password", false)
				token := tc.GetTestJWT(user.ID)
				return user.ID, token
			},
			input: models.UpdateUserRequest{
				RoleID: nil, // Will be set in the test
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
			errMsg:     "only admins can change roles",
		},
		{
			name: "Success_AdminUpdatingOtherUser",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				targetUser := tc.CreateTestUser("target_user", "target@example.com", "test_password", false)
				admin := tc.CreateTestUser("admin_user", "admin@example.com", "test_password", true)
				token := tc.GetTestJWT(admin.ID)
				return targetUser.ID, token
			},
			input: models.UpdateUserRequest{
				Email:  testutil.String("new@example.com"),
				RoleID: nil, // Will be set in the test
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Error_InvalidUserID",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return uuid.UUID{}, token // Return invalid user ID
			},
			input: models.UpdateUserRequest{
				Email: testutil.String("new@example.com"),
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "invalid user id",
		},
		{
			name: "Error_InvalidJSON",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return user.ID, token
			},
			invalidJSON: true,
			wantStatus:  http.StatusBadRequest,
			wantErr:     true,
			errMsg:      "invalid request body",
		},
		{
			name: "Success_AdminUpdatingPassword",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				targetUser := tc.CreateTestUser("target_user", "target@example.com", "test_password", false)
				admin := tc.CreateTestUser("admin_user", "admin@example.com", "test_password", true)
				token := tc.GetTestJWT(admin.ID)
				return targetUser.ID, token
			},
			input: models.UpdateUserRequest{
				Password: testutil.String("new_password123"),
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Error_NonAdminUpdatingPassword",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "test_password", false)
				token := tc.GetTestJWT(user.ID)
				return user.ID, token
			},
			input: models.UpdateUserRequest{
				Password: testutil.String("new_password123"),
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
			errMsg:     "only admins can change passwords via this endpoint",
		},
		{
			name: "Error_DuplicateEmail",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				// Create first user with email we'll try to duplicate
				tc.CreateTestUser("existing_user", "existing@example.com", "password123", false)

				// Create second user who will try to update to the existing email
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return user.ID, token
			},
			input: models.UpdateUserRequest{
				Email: testutil.String("existing@example.com"),
			},
			wantStatus: http.StatusConflict,
			wantErr:    true,
			errMsg:     "email already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			userID, token := tt.setupFunc(tc)

			// Set role ID if needed
			if tt.input.RoleID == nil && (tt.name == "Error_NonAdminUpdatingRole" || tt.name == "Success_AdminUpdatingOtherUser") {
				tt.input.RoleID = tc.GetRoleID("admin")
			}

			var body []byte
			var err error
			if tt.invalidJSON {
				body = []byte(`{"email": invalid}`) // Intentionally malformed JSON
			} else {
				body, err = json.Marshal(tt.input)
				require.NoError(t, err)
			}

			// Create handler and router
			handler := handlers.NewUserHandler(tc.UserRepo, tc.AuthService, tc.PasswordHistoryRepo, tc.AuditRepo)
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router := gin.New()
			router.Use(authMiddleware.AuthRequired())
			router.PUT("/api/v1/users/:id", handler.UpdateUser)

			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/users/%s", userID), bytes.NewReader(body))
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, tt.wantStatus, w.Code)

			if tt.wantErr {
				var resp models.ErrorResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				require.Equal(t, tt.errMsg, resp.Error)
				return
			}

			var resp models.User
			err = json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			require.Equal(t, userID, resp.ID)
			if tt.input.Email != nil {
				require.NotNil(t, resp.Email)
				require.Equal(t, *tt.input.Email, *resp.Email)
			}
			if tt.input.RoleID != nil {
				require.Equal(t, *tt.input.RoleID, resp.RoleID)
			}
		})
	}
}

type getUserTest struct {
	name       string
	setupFunc  func(*testutil.TestContext) (uuid.UUID, string)
	wantStatus int
	wantErr    bool
	errMsg     string
}

func TestUserHandler_GetUser(t *testing.T) {
	tests := []getUserTest{
		{
			name: "Success",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return user.ID, token
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Error_InvalidUserID",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return uuid.UUID{}, token // Return invalid user ID
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "invalid user id",
		},
		{
			name: "Error_UserNotFound",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return uuid.New(), token // Return a random UUID that doesn't exist in the database
			},
			wantStatus: http.StatusNotFound,
			wantErr:    true,
			errMsg:     "user not found",
		},
		{
			name: "Error_NonAdminGettingOtherUser",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				// Create target user
				targetUser := tc.CreateTestUser("target_user", "target@example.com", "test_password", false) // Target user as non-admin

				// Create another non-admin user and get their token
				otherUser := tc.CreateTestUser("other_user", "other@example.com", "test_password", false) // Other user as non-admin
				token := tc.GetTestJWT(otherUser.ID)

				return targetUser.ID, token
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
			errMsg:     "permission denied",
		},
		{
			name: "Success_AdminGettingOtherUser",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				// Create target user
				targetUser := tc.CreateTestUser("target_user", "target@example.com", "test_password", false)

				// Create admin user and get their token
				admin := tc.CreateTestUser("admin_user", "admin@example.com", "test_password", true) // Create as admin
				token := tc.GetTestJWT(admin.ID)

				return targetUser.ID, token
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			userID, token := tt.setupFunc(tc)

			// Create handler and router
			handler := handlers.NewUserHandler(tc.UserRepo, tc.AuthService, tc.PasswordHistoryRepo, tc.AuditRepo)
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router := gin.New()
			router.Use(authMiddleware.AuthRequired())
			router.GET("/api/v1/users/:id", handler.GetUser)

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/users/%s", userID), nil)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, tt.wantStatus, w.Code)

			if tt.wantErr {
				var resp models.ErrorResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				require.Equal(t, tt.errMsg, resp.Error)
				return
			}

			var resp models.User
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			require.Equal(t, userID, resp.ID)
		})
	}
}

func TestUserHandler_ChangePassword(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.TestContext) (uuid.UUID, string) // returns userID and token
		input      models.ChangePasswordRequest
		wantStatus int
		wantErr    bool
		errMsg     string
	}{
		{
			name: "Success",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return user.ID, token
			},
			input: models.ChangePasswordRequest{
				CurrentPassword: "password123",
				NewPassword:     "new_password123",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Error_InvalidCurrentPassword",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return user.ID, token
			},
			input: models.ChangePasswordRequest{
				CurrentPassword: "wrong_password",
				NewPassword:     "new_password123",
			},
			wantStatus: http.StatusUnauthorized,
			wantErr:    true,
			errMsg:     "invalid current password",
		},
		{
			name: "Error_PasswordReuse",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)

				// Add current password to history
				hashedPassword, err := tc.AuthService.HashPassword("password123")
				require.NoError(tc.T, err)
				err = tc.PasswordHistoryRepo.Add(context.Background(), user.ID, hashedPassword)
				require.NoError(tc.T, err)

				token := tc.GetTestJWT(user.ID)
				return user.ID, token
			},
			input: models.ChangePasswordRequest{
				CurrentPassword: "password123",
				NewPassword:     "password123",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "password was recently used",
		},
		{
			name: "Error_UserNotFound",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				// Return a random UUID that doesn't exist in the database
				return uuid.New(), token
			},
			input: models.ChangePasswordRequest{
				CurrentPassword: "password123",
				NewPassword:     "new_password123",
			},
			wantStatus: http.StatusNotFound,
			wantErr:    true,
			errMsg:     "user not found",
		},
		{
			name: "Error_WeakPassword",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return user.ID, token
			},
			input: models.ChangePasswordRequest{
				CurrentPassword: "password123",
				NewPassword:     "weak",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "Key: 'ChangePasswordRequest.NewPassword' Error:Field validation for 'NewPassword' failed on the 'min' tag",
		},
		{
			name: "Error_NonAdminChangingOtherUserPassword",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				// Create target user
				targetUser := tc.CreateTestUser("target_user", "target@example.com", "password123", false)

				// Create another user and get their token
				otherUser := tc.CreateTestUser("other_user", "other@example.com", "password123", false)
				token := tc.GetTestJWT(otherUser.ID)

				return targetUser.ID, token
			},
			input: models.ChangePasswordRequest{
				CurrentPassword: "password123",
				NewPassword:     "new_password123",
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
			errMsg:     "permission denied",
		},
		{
			name: "Error_AdminUsingPasswordChangeEndpoint",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin_user", "admin@example.com", "password123", true)
				token := tc.GetTestJWT(admin.ID)
				return admin.ID, token
			},
			input: models.ChangePasswordRequest{
				CurrentPassword: "password123",
				NewPassword:     "new_password123",
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
			errMsg:     "admins must use the user update endpoint to change passwords",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			// Setup and get user ID and token
			userID, token := tt.setupFunc(tc)

			// Create handler
			handler := handlers.NewUserHandler(tc.UserRepo, tc.AuthService, tc.PasswordHistoryRepo, tc.AuditRepo)
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)

			// Create request
			body, err := json.Marshal(tt.input)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/users/%s/password", userID), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			// Create response recorder
			w := httptest.NewRecorder()

			// Setup router and middleware
			router := gin.New()
			router.Use(authMiddleware.AuthRequired())
			router.PUT("/users/:id/password", handler.ChangePassword)

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

			// Verify password was changed
			user, err := tc.UserRepo.GetByID(context.Background(), userID)
			require.NoError(t, err)
			err = tc.AuthService.ComparePasswords(user.Password, tt.input.NewPassword)
			require.NoError(t, err)
		})
	}
}

type listUsersTest struct {
	name         string
	setupFunc    func(*testutil.TestContext) string
	wantStatus   int
	wantCount    int
	validateUser func(*testing.T, models.User)
}

func TestUserHandler_ListUsers(t *testing.T) {
	tests := []listUsersTest{
		{
			name: "Success_AdminListsAllUsers",
			setupFunc: func(tc *testutil.TestContext) string {
				// Create admin user
				admin := tc.CreateTestUser("admin", "admin@example.com", "password123", true)
				// Create additional test users
				tc.CreateTestUser("user1", "user1@example.com", "password123", false)
				tc.CreateTestUser("user2", "user2@example.com", "password123", false)
				return tc.GetTestJWT(admin.ID)
			},
			wantStatus: http.StatusOK,
			wantCount:  3, // Admin + 2 regular users
		},
		{
			name: "Success_NonAdminListsOwnAccount",
			setupFunc: func(tc *testutil.TestContext) string {
				// Create regular user
				user := tc.CreateTestUser("user1", "user1@example.com", "password123", false)
				// Create additional test users that shouldn't be visible
				tc.CreateTestUser("user2", "user2@example.com", "password123", false)
				tc.CreateTestUser("user3", "user3@example.com", "password123", false)
				return tc.GetTestJWT(user.ID)
			},
			wantStatus: http.StatusOK,
			wantCount:  1, // Only their own account
			validateUser: func(t *testing.T, user models.User) {
				require.Equal(t, "user1", user.Username)
				require.Equal(t, "user1@example.com", *user.Email)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			token := tt.setupFunc(tc)

			// Create handler and router
			handler := handlers.NewUserHandler(tc.UserRepo, tc.AuthService, tc.PasswordHistoryRepo, tc.AuditRepo)
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router := gin.New()
			router.Use(authMiddleware.AuthRequired())
			router.GET("/api/v1/users", handler.ListUsers)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, tt.wantStatus, w.Code)

			var users []models.User
			err := json.NewDecoder(w.Body).Decode(&users)
			require.NoError(t, err)
			require.Len(t, users, tt.wantCount)

			if tt.validateUser != nil && len(users) > 0 {
				tt.validateUser(t, users[0])
			}
		})
	}
}

func TestUserHandler_Register(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.TestContext) string // returns token if needed
		input      models.CreateUserRequest
		regEnabled bool
		wantStatus int
		wantErr    bool
		errMsg     string
		validate   func(*testing.T, *testutil.TestContext, models.User)
	}{
		{
			name: "Success_FirstUserGetsAdminRole",
			setupFunc: func(tc *testutil.TestContext) string {
				return "" // No token needed for first registration
			},
			input: models.CreateUserRequest{
				Username: "first_user",
				Email:    testutil.String("first@example.com"),
				Password: "password123",
			},
			regEnabled: true,
			wantStatus: http.StatusCreated,
			validate: func(t *testing.T, tc *testutil.TestContext, user models.User) {
				// Get admin role
				adminRole, err := tc.RoleRepo.GetByName(context.Background(), "admin")
				require.NoError(t, err)
				require.Equal(t, adminRole.ID, user.RoleID)
			},
		},
		{
			name: "Error_RegistrationDisabled",
			setupFunc: func(tc *testutil.TestContext) string {
				// Create first user so this isn't treated as first user registration
				tc.CreateTestUser("first_user", "first@example.com", "password123", true)
				return "" // No token for unauthenticated request
			},
			input: models.CreateUserRequest{
				Username: "new_user",
				Email:    testutil.String("new@example.com"),
				Password: "password123",
			},
			regEnabled: false,
			wantStatus: http.StatusForbidden,
			wantErr:    true,
			errMsg:     "registration is disabled",
		},
		{
			name: "Success_AdminCanCreateUserWhenRegDisabled",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin_user", "admin@example.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateUserRequest{
				Username: "new_user",
				Email:    testutil.String("new@example.com"),
				Password: "password123",
			},
			regEnabled: false,
			wantStatus: http.StatusCreated,
			validate: func(t *testing.T, tc *testutil.TestContext, user models.User) {
				// Should get regular user role
				userRole, err := tc.RoleRepo.GetByName(context.Background(), "user")
				require.NoError(t, err)
				require.Equal(t, userRole.ID, user.RoleID)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			// Set registration enabled/disabled in config
			tc.Config.Auth.RegistrationOpen = tt.regEnabled

			// Get token from setup if provided
			token := ""
			if tt.setupFunc != nil {
				token = tt.setupFunc(tc)
			}

			// Create request
			body, err := json.Marshal(tt.input)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			if token != "" {
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			}
			w := httptest.NewRecorder()

			// Setup router and make request
			router := gin.New()
			if token != "" {
				authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
				router.Use(authMiddleware.AuthRequired())
			}
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
					tt.validate(t, tc, user)
				}
			}
		})
	}
}

type deleteUserTest struct {
	name       string
	setupFunc  func(*testutil.TestContext) (uuid.UUID, string)
	wantStatus int
	wantErr    bool
	errMsg     string
}

func TestUserHandler_DeleteUser(t *testing.T) {
	tests := []deleteUserTest{
		{
			name: "Success_DeleteOwnAccount",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return user.ID, token
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Success_AdminDeletingOtherUser",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				// Create target user to be deleted
				targetUser := tc.CreateTestUser("target_user", "target@example.com", "password123", false)
				// Create admin user
				admin := tc.CreateTestUser("admin_user", "admin@example.com", "password123", true)
				token := tc.GetTestJWT(admin.ID)
				return targetUser.ID, token
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Error_NonAdminDeletingOtherUser",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				// Create target user to be deleted
				targetUser := tc.CreateTestUser("target_user", "target@example.com", "password123", false)
				// Create non-admin user
				user := tc.CreateTestUser("other_user", "other@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return targetUser.ID, token
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
			errMsg:     "permission denied - can only delete own account unless admin",
		},
		{
			name: "Error_UserNotFound",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return uuid.New(), token // Return a random UUID that doesn't exist
			},
			wantStatus: http.StatusNotFound,
			wantErr:    true,
			errMsg:     "user not found",
		},
		{
			name: "Error_InvalidUserID",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "password123", false)
				token := tc.GetTestJWT(user.ID)
				return uuid.Nil, token // Return invalid UUID
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "invalid user id",
		},
		{
			name: "Error_DeleteAdminUser",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				// Create an admin user that we'll try to delete
				admin := tc.CreateTestUser("admin_user", "admin@example.com", "password123", true)
				// Create another admin to perform the deletion
				otherAdmin := tc.CreateTestUser("other_admin", "other@example.com", "password123", true)
				token := tc.GetTestJWT(otherAdmin.ID)
				return admin.ID, token
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "cannot delete admin user",
		},
		{
			name: "Error_AdminDeletingSelf",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				// Create an admin user
				admin := tc.CreateTestUser("admin_user", "admin@example.com", "test_password", true)
				token := tc.GetTestJWT(admin.ID)
				return admin.ID, token // Return admin's own ID and token
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "cannot delete admin user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			userID, token := tt.setupFunc(tc)

			// Create handler and router
			handler := handlers.NewUserHandler(tc.UserRepo, tc.AuthService, tc.PasswordHistoryRepo, tc.AuditRepo)
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router := gin.New()
			router.Use(authMiddleware.AuthRequired())
			router.DELETE("/api/v1/users/:id", handler.DeleteUser)

			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/users/%s", userID), nil)
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, tt.wantStatus, w.Code)

			if tt.wantErr {
				var resp models.ErrorResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				require.Equal(t, tt.errMsg, resp.Error)
				return
			}

			var resp models.SuccessResponse
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			require.Equal(t, "user deleted successfully", resp.Message)

			// Verify user was actually deleted
			_, err = tc.UserRepo.GetByID(context.Background(), userID)
			require.Equal(t, repository.ErrUserNotFound, err)
		})
	}
}
