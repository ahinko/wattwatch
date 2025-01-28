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

func TestRoleRepository_Create(t *testing.T) {
	tc := integration.NewTestContext(t)

	tests := []struct {
		name    string
		role    *models.Role
		wantErr error
	}{
		{
			name: "Success",
			role: &models.Role{
				Name:         "test-role",
				IsProtected:  false,
				IsAdminGroup: false,
			},
		},
		{
			name: "Duplicate Name",
			role: &models.Role{
				Name:         "test-role",
				IsProtected:  false,
				IsAdminGroup: false,
			},
			wantErr: repository.ErrConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tc.RoleRepo.Create(context.Background(), tt.role)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotEqual(t, uuid.Nil, tt.role.ID)
			require.False(t, tt.role.CreatedAt.IsZero())
			require.False(t, tt.role.UpdatedAt.IsZero())

			// Verify role was created
			var exists bool
			err = tc.DB.QueryRowContext(context.Background(),
				"SELECT EXISTS(SELECT 1 FROM roles WHERE id = $1 AND name = $2 AND is_protected = $3 AND is_admin_group = $4)",
				tt.role.ID, tt.role.Name, tt.role.IsProtected, tt.role.IsAdminGroup).Scan(&exists)
			require.NoError(t, err)
			require.True(t, exists)
		})
	}
}

func TestRoleRepository_Update(t *testing.T) {
	tc := integration.NewTestContext(t)

	// Create initial role
	role := &models.Role{
		Name:         "test-role",
		IsProtected:  false,
		IsAdminGroup: false,
	}
	require.NoError(t, tc.RoleRepo.Create(context.Background(), role))

	// Create protected role
	protectedRole := &models.Role{
		Name:         "protected-role",
		IsProtected:  true,
		IsAdminGroup: false,
	}
	require.NoError(t, tc.RoleRepo.Create(context.Background(), protectedRole))

	// Create another role for duplicate name test
	otherRole := &models.Role{
		Name:         "other-role",
		IsProtected:  false,
		IsAdminGroup: false,
	}
	require.NoError(t, tc.RoleRepo.Create(context.Background(), otherRole))

	tests := []struct {
		name    string
		role    *models.Role
		wantErr error
	}{
		{
			name: "Success",
			role: &models.Role{
				ID:           role.ID,
				Name:         "updated-role",
				IsProtected:  false,
				IsAdminGroup: true,
			},
		},
		{
			name: "Non-existent ID",
			role: &models.Role{
				ID:           uuid.New(),
				Name:         "non-existent",
				IsProtected:  false,
				IsAdminGroup: false,
			},
			wantErr: repository.ErrNotFound,
		},
		{
			name: "Protected Role",
			role: &models.Role{
				ID:           protectedRole.ID,
				Name:         "updated-protected",
				IsProtected:  true,
				IsAdminGroup: false,
			},
			wantErr: repository.ErrProtectedRole,
		},
		{
			name: "Duplicate Name",
			role: &models.Role{
				ID:           role.ID,
				Name:         otherRole.Name,
				IsProtected:  false,
				IsAdminGroup: false,
			},
			wantErr: repository.ErrConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tc.RoleRepo.Update(context.Background(), tt.role)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.False(t, tt.role.UpdatedAt.IsZero())

			// Verify update
			var exists bool
			err = tc.DB.QueryRowContext(context.Background(),
				"SELECT EXISTS(SELECT 1 FROM roles WHERE id = $1 AND name = $2 AND is_protected = $3 AND is_admin_group = $4)",
				tt.role.ID, tt.role.Name, tt.role.IsProtected, tt.role.IsAdminGroup).Scan(&exists)
			require.NoError(t, err)
			require.True(t, exists)
		})
	}
}

func TestRoleRepository_Delete(t *testing.T) {
	tc := integration.NewTestContext(t)

	tests := []struct {
		name    string
		setup   func() uuid.UUID
		wantErr error
	}{
		{
			name: "Success",
			setup: func() uuid.UUID {
				role := &models.Role{
					Name:         "test-role",
					IsProtected:  false,
					IsAdminGroup: false,
				}
				err := tc.RoleRepo.Create(context.Background(), role)
				require.NoError(t, err)
				return role.ID
			},
		},
		{
			name: "Non-existent ID",
			setup: func() uuid.UUID {
				return uuid.New()
			},
			wantErr: repository.ErrNotFound,
		},
		{
			name: "Protected Role",
			setup: func() uuid.UUID {
				role := &models.Role{
					Name:         "test-protected",
					IsProtected:  true,
					IsAdminGroup: false,
				}
				err := tc.RoleRepo.Create(context.Background(), role)
				require.NoError(t, err)
				return role.ID
			},
			wantErr: repository.ErrProtectedRole,
		},
		{
			name: "Role With Users",
			setup: func() uuid.UUID {
				role := &models.Role{
					Name:         "test-role-with-users",
					IsProtected:  false,
					IsAdminGroup: false,
				}
				err := tc.RoleRepo.Create(context.Background(), role)
				require.NoError(t, err)

				user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)
				_, err = tc.DB.ExecContext(context.Background(),
					"UPDATE users SET role_id = $1 WHERE id = $2",
					role.ID, user.ID)
				require.NoError(t, err)

				return role.ID
			},
			wantErr: repository.ErrHasAssociatedRecords,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roleID := tt.setup()
			err := tc.RoleRepo.Delete(context.Background(), roleID)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			// Verify role was deleted
			var deletedAt *time.Time
			err = tc.DB.QueryRowContext(context.Background(),
				"SELECT deleted_at FROM roles WHERE id = $1",
				roleID).Scan(&deletedAt)
			require.NoError(t, err)
			require.NotNil(t, deletedAt)
			require.WithinDuration(t, time.Now(), *deletedAt, 2*time.Second)

			// Verify role is not returned by GetByID
			_, err = tc.RoleRepo.GetByID(context.Background(), roleID)
			require.ErrorIs(t, err, repository.ErrNotFound)
		})
	}
}

func TestRoleRepository_GetByID(t *testing.T) {
	tc := integration.NewTestContext(t)

	// Create test role
	role := &models.Role{
		Name:         "test-role",
		IsProtected:  false,
		IsAdminGroup: false,
	}
	require.NoError(t, tc.RoleRepo.Create(context.Background(), role))

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr error
	}{
		{
			name: "Success",
			id:   role.ID,
		},
		{
			name:    "Non-existent ID",
			id:      uuid.New(),
			wantErr: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tc.RoleRepo.GetByID(context.Background(), tt.id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, role.ID, result.ID)
			require.Equal(t, role.Name, result.Name)
			require.Equal(t, role.IsProtected, result.IsProtected)
			require.Equal(t, role.IsAdminGroup, result.IsAdminGroup)
			require.False(t, result.CreatedAt.IsZero())
			require.False(t, result.UpdatedAt.IsZero())
		})
	}
}

func TestRoleRepository_GetByName(t *testing.T) {
	tc := integration.NewTestContext(t)

	// Create test role
	role := &models.Role{
		Name:         "test-role",
		IsProtected:  false,
		IsAdminGroup: false,
	}
	require.NoError(t, tc.RoleRepo.Create(context.Background(), role))

	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{
			name:  "Success",
			input: role.Name,
		},
		{
			name:    "Non-existent Name",
			input:   "non-existent",
			wantErr: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tc.RoleRepo.GetByName(context.Background(), tt.input)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, role.ID, result.ID)
			require.Equal(t, role.Name, result.Name)
			require.Equal(t, role.IsProtected, result.IsProtected)
			require.Equal(t, role.IsAdminGroup, result.IsAdminGroup)
			require.False(t, result.CreatedAt.IsZero())
			require.False(t, result.UpdatedAt.IsZero())
		})
	}
}

func TestRoleRepository_List(t *testing.T) {
	tc := integration.NewTestContext(t)

	// Create test roles
	roles := []*models.Role{
		{
			Name:         "test-role-1",
			IsProtected:  false,
			IsAdminGroup: false,
		},
		{
			Name:         "test-role-2",
			IsProtected:  true,
			IsAdminGroup: false,
		},
		{
			Name:         "test-role-3",
			IsProtected:  false,
			IsAdminGroup: true,
		},
	}

	for _, role := range roles {
		err := tc.RoleRepo.Create(context.Background(), role)
		require.NoError(t, err)
	}

	searchStr := "test-role-1"
	protectedTrue := true
	adminGroupTrue := true

	tests := []struct {
		name      string
		filter    repository.RoleFilter
		wantCount int
		wantErr   error
	}{
		{
			name:      "No Filter",
			filter:    repository.RoleFilter{},
			wantCount: 5, // 3 test roles + admin + user
		},
		{
			name: "Search Filter",
			filter: repository.RoleFilter{
				Search: &searchStr,
			},
			wantCount: 1,
		},
		{
			name: "Protected Filter",
			filter: repository.RoleFilter{
				Protected: &protectedTrue,
			},
			wantCount: 3, // test-role-2 + admin + user
		},
		{
			name: "Admin Group Filter",
			filter: repository.RoleFilter{
				AdminGroup: &adminGroupTrue,
			},
			wantCount: 2, // test-role-3 + admin
		},
		{
			name: "Order By Name DESC",
			filter: repository.RoleFilter{
				OrderBy:   "name",
				OrderDesc: true,
			},
			wantCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := tc.RoleRepo.List(context.Background(), tt.filter)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, results, tt.wantCount)

			// Verify each result has required fields
			for _, result := range results {
				require.NotEqual(t, uuid.Nil, result.ID)
				require.NotEmpty(t, result.Name)
				require.False(t, result.CreatedAt.IsZero())
				require.False(t, result.UpdatedAt.IsZero())
			}
		})
	}
}
