package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"wattwatch/internal/api/handlers"
	"wattwatch/internal/api/middleware"
	"wattwatch/internal/models"
	"wattwatch/internal/testutil"

	"github.com/google/uuid"
)

func TestRoleHandler_CreateRole(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.TestContext) string
		input      models.CreateRoleRequest
		wantStatus int
		wantErr    bool
		errMsg     string
	}{
		{
			name: "Success_BasicRole",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateRoleRequest{
				Name: "test_role",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "Success_SpecialCharacters",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateRoleRequest{
				Name: "test-role@123_special",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "Error_EmptyName",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateRoleRequest{
				Name: "",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "Key: 'CreateRoleRequest.Name' Error:Field validation for 'Name' failed on the 'required' tag",
		},
		{
			name: "Error_NameTooShort",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateRoleRequest{
				Name: "x",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "Key: 'CreateRoleRequest.Name' Error:Field validation for 'Name' failed on the 'min' tag",
		},
		{
			name: "Error_NameTooLong",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateRoleRequest{
				Name: "this_is_a_very_long_role_name_that_exceeds_the_maximum_length_allowed_for_role_names",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "Key: 'CreateRoleRequest.Name' Error:Field validation for 'Name' failed on the 'max' tag",
		},
		{
			name: "Error_SpacesOnly",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateRoleRequest{
				Name: "   ",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "Key: 'CreateRoleRequest.Name' Error:Field validation for 'Name' failed on the 'nospaces' tag",
		},
		{
			name: "Error_NonAdmin",
			setupFunc: func(tc *testutil.TestContext) string {
				user := tc.CreateTestUser("test_user", "test@example.com", "test_password", false)
				return tc.GetTestJWT(user.ID)
			},
			input: models.CreateRoleRequest{
				Name: "test_role",
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
			errMsg:     "permission denied",
		},
		{
			name: "Error_DuplicateName",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)

				// Create a role with the same name first
				err := tc.RoleRepo.Create(context.Background(), &models.Role{
					Name: "test_role",
				})
				require.NoError(tc.T, err)

				return tc.GetTestJWT(admin.ID)
			},
			input: models.CreateRoleRequest{
				Name: "test_role",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "role name already exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			token := tt.setupFunc(tc)

			handler := handlers.NewRoleHandler(tc.RoleRepo, tc.UserRepo, tc.AuditRepo)
			router := gin.New()
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router.Use(authMiddleware.AuthRequired())
			router.POST("/api/v1/roles", handler.CreateRole)

			body, err := json.Marshal(tt.input)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/roles", bytes.NewReader(body))
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

			var resp models.Role
			err = json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			require.Equal(t, tt.input.Name, resp.Name)
			require.False(t, resp.IsProtected)
		})
	}
}

func TestRoleHandler_UpdateRole(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.TestContext) (uuid.UUID, string)
		input      models.UpdateRoleRequest
		wantStatus int
		wantErr    bool
		errMsg     string
	}{
		{
			name: "Success_BasicUpdate",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				role := tc.CreateTestRole("test-role", false, false)
				return role.ID, tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateRoleRequest{
				Name: "updated_role",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Success_SpecialCharacters",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				role := tc.CreateTestRole("test-role@123_special", false, false)
				return role.ID, tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateRoleRequest{
				Name: "updated-role@123_special",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Error_EmptyName",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				role := tc.CreateTestRole("test-role@123_special", false, false)
				return role.ID, tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateRoleRequest{
				Name: "",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "Key: 'UpdateRoleRequest.Name' Error:Field validation for 'Name' failed on the 'required' tag",
		},
		{
			name: "Error_NameTooShort",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				role := tc.CreateTestRole("test-role@123_special", false, false)
				return role.ID, tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateRoleRequest{
				Name: "x",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "Key: 'UpdateRoleRequest.Name' Error:Field validation for 'Name' failed on the 'min' tag",
		},
		{
			name: "Error_NameTooLong",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				role := tc.CreateTestRole("test-role@123_special", false, false)
				return role.ID, tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateRoleRequest{
				Name: "this_is_a_very_long_role_name_that_exceeds_the_maximum_length_allowed_for_role_names",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "Key: 'UpdateRoleRequest.Name' Error:Field validation for 'Name' failed on the 'max' tag",
		},
		{
			name: "Error_SpacesOnly",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				role := tc.CreateTestRole("test-role@123_special", false, false)
				return role.ID, tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateRoleRequest{
				Name: "   ",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "Key: 'UpdateRoleRequest.Name' Error:Field validation for 'Name' failed on the 'nospaces' tag",
		},
		{
			name: "Error_InvalidID",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return uuid.UUID{}, tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateRoleRequest{
				Name: "updated_role",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "invalid role ID",
		},
		{
			name: "Error_NonAdmin",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "test_password", false)
				role := tc.CreateTestRole("test-role@123_special", false, false)
				return role.ID, tc.GetTestJWT(user.ID)
			},
			input: models.UpdateRoleRequest{
				Name: "updated_role",
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
			errMsg:     "permission denied",
		},
		{
			name: "Error_ProtectedRole",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)

				// Get the protected admin role
				role, err := tc.RoleRepo.GetByName(context.Background(), "admin")
				require.NoError(tc.T, err)

				return role.ID, tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateRoleRequest{
				Name: "updated_admin",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "cannot modify protected role",
		},
		{
			name: "Error_DuplicateName",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)

				// Create two roles
				role1 := &models.Role{
					Name: "role_1",
				}
				err := tc.RoleRepo.Create(context.Background(), role1)
				require.NoError(tc.T, err)

				role2 := &models.Role{
					Name: "role_2",
				}
				err = tc.RoleRepo.Create(context.Background(), role2)
				require.NoError(tc.T, err)

				// Try to update role2's name to role1's name
				return role2.ID, tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateRoleRequest{
				Name: "role_1",
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "role name already exists",
		},
		{
			name: "Error_RoleNotFound",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return uuid.New(), tc.GetTestJWT(admin.ID)
			},
			input: models.UpdateRoleRequest{
				Name: "updated_role",
			},
			wantStatus: http.StatusNotFound,
			wantErr:    true,
			errMsg:     "role not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			roleID, token := tt.setupFunc(tc)

			handler := handlers.NewRoleHandler(tc.RoleRepo, tc.UserRepo, tc.AuditRepo)
			router := gin.New()
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router.Use(authMiddleware.AuthRequired())
			router.PUT("/api/v1/roles/:id", handler.UpdateRole)

			body, err := json.Marshal(tt.input)
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/roles/%s", roleID), bytes.NewReader(body))
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

			var resp models.Role
			err = json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			require.Equal(t, tt.input.Name, resp.Name)
		})
	}
}

func TestRoleHandler_DeleteRole(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.TestContext) (uuid.UUID, string)
		wantStatus int
		wantErr    bool
		errMsg     string
	}{
		{
			name: "Success",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				role := tc.CreateTestRole("test-role", false, false)
				return role.ID, tc.GetTestJWT(admin.ID)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Error_NonAdmin",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "test_password", false)
				role := tc.CreateTestRole("test-role", false, false)
				return role.ID, tc.GetTestJWT(user.ID)
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
			errMsg:     "permission denied",
		},
		{
			name: "Error_ProtectedRole",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)

				// Get the protected admin role
				role, err := tc.RoleRepo.GetByName(context.Background(), "admin")
				require.NoError(tc.T, err)

				return role.ID, tc.GetTestJWT(admin.ID)
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "cannot delete protected role",
		},
		{
			name: "Error_RoleInUse",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)

				// Create a role and assign it to a user
				role := &models.Role{
					Name: "test_role",
				}
				err := tc.RoleRepo.Create(context.Background(), role)
				require.NoError(tc.T, err)

				user := tc.CreateTestUser("test_user", "test@example.com", "test_password", false)
				user.RoleID = role.ID
				err = tc.UserRepo.Update(context.Background(), user)
				require.NoError(tc.T, err)

				return role.ID, tc.GetTestJWT(admin.ID)
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			errMsg:     "cannot delete role with assigned users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			roleID, token := tt.setupFunc(tc)

			handler := handlers.NewRoleHandler(tc.RoleRepo, tc.UserRepo, tc.AuditRepo)
			router := gin.New()
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router.Use(authMiddleware.AuthRequired())
			router.DELETE("/api/v1/roles/:id", handler.DeleteRole)

			req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/roles/%s", roleID), nil)
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

			// Verify role was deleted
			_, err := tc.RoleRepo.GetByID(context.Background(), roleID)
			require.Error(t, err)
		})
	}
}

func TestRoleHandler_ListRoles(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.TestContext) string
		wantStatus int
		wantErr    bool
		errMsg     string
		wantCount  int
		validate   func(*testing.T, []models.Role)
	}{
		{
			name: "Success_AdminListsAllRoles",
			setupFunc: func(tc *testutil.TestContext) string {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)

				// Create some test roles
				for i := 1; i <= 3; i++ {
					role := &models.Role{
						Name: fmt.Sprintf("test_role_%d", i),
					}
					err := tc.RoleRepo.Create(context.Background(), role)
					require.NoError(tc.T, err)
				}

				return tc.GetTestJWT(admin.ID)
			},
			wantStatus: http.StatusOK,
			wantCount:  5, // 2 default roles (admin, user) + 3 created roles
		},
		{
			name: "Success_NonAdminListsOwnRole",
			setupFunc: func(tc *testutil.TestContext) string {
				user := tc.CreateTestUser("test_user", "test@example.com", "test_password", false)
				return tc.GetTestJWT(user.ID)
			},
			wantStatus: http.StatusOK,
			wantCount:  1, // Only their own role
			validate: func(t *testing.T, roles []models.Role) {
				require.Len(t, roles, 1)
				require.Equal(t, "user", roles[0].Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			token := tt.setupFunc(tc)

			handler := handlers.NewRoleHandler(tc.RoleRepo, tc.UserRepo, tc.AuditRepo)
			router := gin.New()
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router.Use(authMiddleware.AuthRequired())
			router.GET("/api/v1/roles", handler.ListRoles)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/roles", nil)
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

			var resp []models.Role
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			require.Len(t, resp, tt.wantCount)

			if tt.validate != nil {
				tt.validate(t, resp)
			}
		})
	}
}

func TestRoleHandler_GetRole(t *testing.T) {
	tests := []struct {
		name       string
		setupFunc  func(*testutil.TestContext) (uuid.UUID, string)
		wantStatus int
		wantErr    bool
		errMsg     string
	}{
		{
			name: "Success",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				role := tc.CreateTestRole("test-role", false, false)
				return role.ID, tc.GetTestJWT(admin.ID)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Error_NonAdmin",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				user := tc.CreateTestUser("test_user", "test@example.com", "test_password", false)
				role := tc.CreateTestRole("test-role", false, false)
				return role.ID, tc.GetTestJWT(user.ID)
			},
			wantStatus: http.StatusForbidden,
			wantErr:    true,
			errMsg:     "permission denied",
		},
		{
			name: "Error_RoleNotFound",
			setupFunc: func(tc *testutil.TestContext) (uuid.UUID, string) {
				admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
				return uuid.New(), tc.GetTestJWT(admin.ID)
			},
			wantStatus: http.StatusNotFound,
			wantErr:    true,
			errMsg:     "role not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := testutil.NewTestContext(t)

			roleID, token := tt.setupFunc(tc)

			handler := handlers.NewRoleHandler(tc.RoleRepo, tc.UserRepo, tc.AuditRepo)
			router := gin.New()
			authMiddleware := middleware.NewAuthMiddleware(tc.AuthService, tc.UserRepo, tc.RoleRepo)
			router.Use(authMiddleware.AuthRequired())
			router.GET("/api/v1/roles/:id", handler.GetRole)

			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/roles/%s", roleID), nil)
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

			var resp models.Role
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			require.Equal(t, roleID, resp.ID)
		})
	}
}
