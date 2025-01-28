package postgres_test

import (
	"context"
	"sort"
	"testing"
	"time"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"
	"wattwatch/internal/repository/postgres"
	"wattwatch/internal/testutil"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUserRepository_Create(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewUserRepository(tc.DB)

	// Clean up any existing test users
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM users WHERE username != 'admin'")
	require.NoError(t, err)

	// Get user role for testing
	userRole, err := tc.RoleRepo.GetByName(context.Background(), "user")
	require.NoError(t, err)

	tests := []struct {
		name    string
		input   models.User
		wantErr bool
		errType error
	}{
		{
			name: "Success",
			input: models.User{
				Username: "testuser",
				Password: "password123",
				Email:    &[]string{"test@example.com"}[0],
				RoleID:   userRole.ID,
			},
		},
		{
			name: "Duplicate Username",
			input: models.User{
				Username: "testuser",
				Password: "password123",
				Email:    &[]string{"test2@example.com"}[0],
				RoleID:   userRole.ID,
			},
			wantErr: true,
		},
		{
			name: "Duplicate Email",
			input: models.User{
				Username: "testuser2",
				Password: "password123",
				Email:    &[]string{"test@example.com"}[0],
				RoleID:   userRole.ID,
			},
			wantErr: true,
		},
		{
			name: "With Role",
			input: models.User{
				Username: "testuser3",
				Password: "password123",
				Email:    &[]string{"test3@example.com"}[0],
				RoleID:   userRole.ID,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(context.Background(), &tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.Equal(t, tt.errType, err)
				}
			} else {
				require.NoError(t, err)
				require.NotEqual(t, uuid.Nil, tt.input.ID)
				require.False(t, tt.input.CreatedAt.IsZero())
				require.False(t, tt.input.UpdatedAt.IsZero())

				// Verify creation
				saved, err := repo.GetByID(context.Background(), tt.input.ID)
				require.NoError(t, err)
				require.Equal(t, tt.input.Username, saved.Username)
				require.Equal(t, tt.input.Email, saved.Email)
				require.Equal(t, tt.input.RoleID, saved.RoleID)
			}
		})
	}
}

func TestUserRepository_Update(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewUserRepository(tc.DB)

	// Clean up any existing test users
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM users WHERE username != 'admin'")
	require.NoError(t, err)

	// Get user role for testing
	userRole, err := tc.RoleRepo.GetByName(context.Background(), "user")
	require.NoError(t, err)

	// Create initial user
	user := &models.User{
		Username: "testuser",
		Password: "password123",
		Email:    &[]string{"test@example.com"}[0],
		RoleID:   userRole.ID,
	}
	require.NoError(t, repo.Create(context.Background(), user))

	// Create another user for duplicate tests
	otherUser := &models.User{
		Username: "otheruser",
		Password: "password123",
		Email:    &[]string{"other@example.com"}[0],
		RoleID:   userRole.ID,
	}
	require.NoError(t, repo.Create(context.Background(), otherUser))

	tests := []struct {
		name    string
		input   models.User
		wantErr bool
		errType error
	}{
		{
			name: "Success",
			input: models.User{
				ID:       user.ID,
				Username: "updateduser",
				Email:    &[]string{"updated@example.com"}[0],
				RoleID:   userRole.ID,
			},
		},
		{
			name: "Non-existent ID",
			input: models.User{
				ID:       uuid.New(),
				Username: "nonexistent",
				Email:    &[]string{"nonexistent@example.com"}[0],
				RoleID:   userRole.ID,
			},
			wantErr: true,
			errType: repository.ErrNotFound,
		},
		{
			name: "Duplicate Username",
			input: models.User{
				ID:       user.ID,
				Username: otherUser.Username,
				Email:    &[]string{"test3@example.com"}[0],
				RoleID:   userRole.ID,
			},
			wantErr: true,
		},
		{
			name: "Duplicate Email",
			input: models.User{
				ID:       user.ID,
				Username: "uniqueuser",
				Email:    otherUser.Email,
				RoleID:   userRole.ID,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Update(context.Background(), &tt.input)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.Equal(t, tt.errType, err)
				}
			} else {
				require.NoError(t, err)
				require.False(t, tt.input.UpdatedAt.IsZero())

				// Verify update
				updated, err := repo.GetByID(context.Background(), tt.input.ID)
				require.NoError(t, err)
				require.Equal(t, tt.input.Username, updated.Username)
				require.Equal(t, tt.input.Email, updated.Email)
				require.Equal(t, tt.input.RoleID, updated.RoleID)
			}
		})
	}
}

func TestUserRepository_Delete(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewUserRepository(tc.DB)

	// Clean up any existing test users
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM users WHERE username != 'admin'")
	require.NoError(t, err)

	// Create users to delete
	user := tc.CreateTestUser("testuser", "test@example.com", "password123", false)
	adminUser := tc.CreateTestUser("adminuser", "admin@example.com", "password123", true)

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr error
	}{
		{
			name:    "Success - Delete User",
			id:      user.ID,
			wantErr: nil,
		},
		{
			name:    "Error - Non-existent ID",
			id:      uuid.New(),
			wantErr: repository.ErrUserNotFound,
		},
		{
			name:    "Error - Admin User",
			id:      adminUser.ID,
			wantErr: repository.ErrAdminDelete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Delete(context.Background(), tt.id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)

			// Verify user was soft deleted
			var deletedAt *time.Time
			err = tc.DB.QueryRowContext(context.Background(),
				"SELECT deleted_at FROM users WHERE id = $1",
				tt.id).Scan(&deletedAt)
			require.NoError(t, err)
			require.NotNil(t, deletedAt)
			require.WithinDuration(t, time.Now(), *deletedAt, 2*time.Second)

			// Verify user is not returned by GetByID
			_, err = repo.GetByID(context.Background(), tt.id)
			require.ErrorIs(t, err, repository.ErrUserNotFound)
		})
	}
}

func TestUserRepository_GetByID(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewUserRepository(tc.DB)

	// Clean up any existing test users
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM users WHERE username != 'admin'")
	require.NoError(t, err)

	// Create user to get
	user := tc.CreateTestUser("testuser", "test@example.com", "password123", false)

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr error
	}{
		{
			name:    "Success - Get User",
			id:      user.ID,
			wantErr: nil,
		},
		{
			name:    "Error - Non-existent ID",
			id:      uuid.New(),
			wantErr: repository.ErrUserNotFound,
		},
		{
			name:    "Error - Zero UUID",
			id:      uuid.Nil,
			wantErr: repository.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetByID(context.Background(), tt.id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, result)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, user.Username, result.Username)
			require.Equal(t, user.Email, result.Email)
			require.Equal(t, user.RoleID, result.RoleID)
		})
	}
}

func TestUserRepository_GetByUsername(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewUserRepository(tc.DB)

	// Clean up any existing test users
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM users WHERE username != 'admin'")
	require.NoError(t, err)

	// Create user to get
	user := tc.CreateTestUser("testuser", "test@example.com", "password123", false)

	tests := []struct {
		name     string
		username string
		wantErr  error
	}{
		{
			name:     "Success - Get User",
			username: user.Username,
			wantErr:  nil,
		},
		{
			name:     "Error - Non-existent Username",
			username: "nonexistent",
			wantErr:  repository.ErrUserNotFound,
		},
		{
			name:     "Error - Empty Username",
			username: "",
			wantErr:  repository.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetByUsername(context.Background(), tt.username)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, result)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, user.Username, result.Username)
			require.Equal(t, user.Email, result.Email)
			require.Equal(t, user.RoleID, result.RoleID)
		})
	}
}

func TestUserRepository_GetByEmail(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewUserRepository(tc.DB)

	// Clean up any existing test users
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM users WHERE username != 'admin'")
	require.NoError(t, err)

	// Create user to get
	user := tc.CreateTestUser("testuser", "test@example.com", "password123", false)

	tests := []struct {
		name    string
		email   string
		wantErr error
	}{
		{
			name:    "Success - Get User",
			email:   *user.Email,
			wantErr: nil,
		},
		{
			name:    "Error - Non-existent Email",
			email:   "nonexistent@example.com",
			wantErr: repository.ErrUserNotFound,
		},
		{
			name:    "Error - Empty Email",
			email:   "",
			wantErr: repository.ErrUserNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := repo.GetByEmail(context.Background(), tt.email)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, result)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, user.Username, result.Username)
			require.Equal(t, user.Email, result.Email)
			require.Equal(t, user.RoleID, result.RoleID)
		})
	}
}

func TestUserRepository_List(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewUserRepository(tc.DB)

	// Clean up existing users
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM users")
	require.NoError(t, err)

	// Get existing roles
	adminRole, err := tc.RoleRepo.GetByName(context.Background(), "admin")
	require.NoError(t, err)
	userRole, err := tc.RoleRepo.GetByName(context.Background(), "user")
	require.NoError(t, err)

	// Create test users
	admin := tc.CreateTestUser("admin", "admin@test.com", "password123", true)
	user1 := tc.CreateTestUser("user1", "user1@test.com", "password123", false)
	user2 := tc.CreateTestUser("user2", "user2@test.com", "password123", false)
	user3 := tc.CreateTestUser("user3", "user3@test.com", "password123", false)
	user4 := tc.CreateTestUser("user4", "user4@test.com", "password123", false)

	// Verify email for one user
	err = repo.VerifyEmail(context.Background(), user1.ID)
	require.NoError(t, err)

	tests := []struct {
		name     string
		filter   repository.UserFilter
		expected []*models.User
	}{
		{
			name:     "Success - List All Users",
			filter:   repository.UserFilter{},
			expected: []*models.User{admin, user1, user2, user3, user4},
		},
		{
			name: "Success - Search by Username",
			filter: repository.UserFilter{
				Search: &[]string{"user"}[0],
			},
			expected: []*models.User{user1, user2, user3, user4},
		},
		{
			name: "Success - Filter by Role (Admin)",
			filter: repository.UserFilter{
				RoleID: &adminRole.ID,
			},
			expected: []*models.User{admin},
		},
		{
			name: "Success - Filter by Role (User)",
			filter: repository.UserFilter{
				RoleID: &userRole.ID,
			},
			expected: []*models.User{user1, user2, user3, user4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users, err := repo.List(context.Background(), tt.filter)
			require.NoError(t, err)
			require.Equal(t, len(tt.expected), len(users), "expected %d users but got %d", len(tt.expected), len(users))

			// Sort both slices by username for consistent comparison
			sort.Slice(users, func(i, j int) bool {
				return users[i].Username < users[j].Username
			})
			sort.Slice(tt.expected, func(i, j int) bool {
				return tt.expected[i].Username < tt.expected[j].Username
			})

			for i := range users {
				require.Equal(t, tt.expected[i].ID, users[i].ID)
				require.Equal(t, tt.expected[i].Username, users[i].Username)
				require.Equal(t, tt.expected[i].Email, users[i].Email)
				require.Equal(t, tt.expected[i].RoleID, users[i].RoleID)
			}
		})
	}
}

func TestUserRepository_UpdateLastLogin(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewUserRepository(tc.DB)

	// Clean up any existing test users
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM users WHERE username != 'admin'")
	require.NoError(t, err)

	// Create user to update
	user := tc.CreateTestUser("testuser", "test@example.com", "password123", false)
	loginTime := time.Now().UTC()

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr error
	}{
		{
			name:    "Success - Update Last Login",
			id:      user.ID,
			wantErr: nil,
		},
		{
			name:    "Error - Non-existent ID",
			id:      uuid.New(),
			wantErr: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.UpdateLastLogin(context.Background(), tt.id, loginTime)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)

			// Verify last login was updated
			updated, err := repo.GetByID(context.Background(), tt.id)
			require.NoError(t, err)
			require.NotNil(t, updated.LastLoginAt)
			require.WithinDuration(t, loginTime, *updated.LastLoginAt, 2*time.Second)
		})
	}
}

func TestUserRepository_UpdateFailedAttempts(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewUserRepository(tc.DB)

	// Clean up any existing test users
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM users WHERE username != 'admin'")
	require.NoError(t, err)

	// Create user to update
	user := tc.CreateTestUser("testuser", "test@example.com", "password123", false)

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr error
	}{
		{
			name:    "Success - Update Failed Attempts",
			id:      user.ID,
			wantErr: nil,
		},
		{
			name:    "Error - Non-existent ID",
			id:      uuid.New(),
			wantErr: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.UpdateFailedAttempts(context.Background(), tt.id, 1)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)

			// Verify failed attempts were updated
			updated, err := repo.GetByID(context.Background(), tt.id)
			require.NoError(t, err)
			require.Equal(t, 1, updated.FailedLoginAttempts)
			require.NotNil(t, updated.LastFailedLogin)
			require.WithinDuration(t, time.Now(), *updated.LastFailedLogin, 2*time.Second)
		})
	}
}

func TestUserRepository_UpdatePassword(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewUserRepository(tc.DB)

	// Clean up any existing test users
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM users WHERE username != 'admin'")
	require.NoError(t, err)

	// Create user to update
	user := tc.CreateTestUser("testuser", "test@example.com", "password123", false)

	tests := []struct {
		name    string
		id      uuid.UUID
		hash    string
		wantErr bool
		errType error
	}{
		{
			name: "Success",
			id:   user.ID,
			hash: "newhash123",
		},
		{
			name:    "Non-existent ID",
			id:      uuid.New(),
			hash:    "newhash123",
			wantErr: true,
			errType: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.UpdatePassword(context.Background(), tt.id, tt.hash)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.Equal(t, tt.errType, err)
				}
			} else {
				require.NoError(t, err)

				// Verify password was updated
				var hash string
				err = tc.DB.QueryRowContext(context.Background(),
					"SELECT password FROM users WHERE id = $1",
					tt.id).Scan(&hash)
				require.NoError(t, err)
				require.Equal(t, tt.hash, hash)
			}
		})
	}
}

func TestUserRepository_VerifyEmail(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewUserRepository(tc.DB)

	// Clean up any existing test users
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM users WHERE username != 'admin'")
	require.NoError(t, err)

	// Create user to verify
	user := tc.CreateTestUser("testuser", "test@example.com", "password123", false)

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr bool
		errType error
	}{
		{
			name: "Success",
			id:   user.ID,
		},
		{
			name:    "Non-existent ID",
			id:      uuid.New(),
			wantErr: true,
			errType: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.VerifyEmail(context.Background(), tt.id)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.Equal(t, tt.errType, err)
				}
			} else {
				require.NoError(t, err)

				// Verify email was verified
				updated, err := repo.GetByID(context.Background(), tt.id)
				require.NoError(t, err)
				require.True(t, updated.EmailVerified)
				require.NotNil(t, updated.PasswordChangedAt)
				require.WithinDuration(t, time.Now(), *updated.PasswordChangedAt, 2*time.Second)
			}
		})
	}
}

func TestUserRepository_IncrementFailedAttempts(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewUserRepository(tc.DB)

	// Clean up any existing test users
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM users WHERE username != 'admin'")
	require.NoError(t, err)

	// Create user to update
	user := tc.CreateTestUser("testuser", "test@example.com", "password123", false)

	tests := []struct {
		name     string
		username string
		wantErr  bool
		errType  error
	}{
		{
			name:     "Success",
			username: user.Username,
		},
		{
			name:     "Non-existent Username",
			username: "nonexistent",
			wantErr:  true,
			errType:  repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.IncrementFailedAttempts(context.Background(), tt.username)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.Equal(t, tt.errType, err)
				}
			} else {
				require.NoError(t, err)

				// Verify failed attempts were incremented
				var attempts int
				err = tc.DB.QueryRowContext(context.Background(),
					"SELECT failed_login_attempts FROM users WHERE username = $1",
					tt.username).Scan(&attempts)
				require.NoError(t, err)
				require.Equal(t, 1, attempts)

				// Increment again and verify
				err = repo.IncrementFailedAttempts(context.Background(), tt.username)
				require.NoError(t, err)
				err = tc.DB.QueryRowContext(context.Background(),
					"SELECT failed_login_attempts FROM users WHERE username = $1",
					tt.username).Scan(&attempts)
				require.NoError(t, err)
				require.Equal(t, 2, attempts)
			}
		})
	}
}

func TestUserRepository_ResetFailedAttempts(t *testing.T) {
	tc := testutil.NewTestContext(t)
	repo := postgres.NewUserRepository(tc.DB)

	// Clean up any existing test users
	_, err := tc.DB.ExecContext(context.Background(), "DELETE FROM users WHERE username != 'admin'")
	require.NoError(t, err)

	// Create user to update
	user := tc.CreateTestUser("testuser", "test@example.com", "password123", false)

	// Set failed attempts
	_, err = tc.DB.ExecContext(context.Background(),
		"UPDATE users SET failed_login_attempts = 3 WHERE username = $1",
		user.Username)
	require.NoError(t, err)

	tests := []struct {
		name     string
		username string
		wantErr  bool
		errType  error
	}{
		{
			name:     "Success",
			username: user.Username,
		},
		{
			name:     "Non-existent Username",
			username: "nonexistent",
			wantErr:  true,
			errType:  repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.ResetFailedAttempts(context.Background(), tt.username)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.Equal(t, tt.errType, err)
				}
			} else {
				require.NoError(t, err)

				// Verify failed attempts were reset
				var attempts int
				err = tc.DB.QueryRowContext(context.Background(),
					"SELECT failed_login_attempts FROM users WHERE username = $1",
					tt.username).Scan(&attempts)
				require.NoError(t, err)
				require.Equal(t, 0, attempts)
			}
		})
	}
}
