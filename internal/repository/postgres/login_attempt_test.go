package postgres_test

import (
	"context"
	"testing"
	"time"
	"wattwatch/internal/repository"
	"wattwatch/internal/repository/postgres/integration"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestLoginAttemptRepository_Create(t *testing.T) {
	tc := integration.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	tests := []struct {
		name      string
		userID    uuid.UUID
		success   bool
		ipAddress string
		wantErr   error
	}{
		{
			name:      "Success - Successful Login",
			userID:    user.ID,
			success:   true,
			ipAddress: "127.0.0.1",
		},
		{
			name:      "Success - Failed Login",
			userID:    user.ID,
			success:   false,
			ipAddress: "192.168.1.1",
		},
		{
			name:      "Invalid User ID",
			userID:    uuid.New(),
			success:   true,
			ipAddress: "127.0.0.1",
			wantErr:   repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tc.LoginAttemptRepo.Create(context.Background(), tt.userID, tt.success, tt.ipAddress, time.Now())
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			// Verify attempt was created
			var exists bool
			err = tc.DB.QueryRowContext(context.Background(),
				"SELECT EXISTS(SELECT 1 FROM login_attempts WHERE user_id = $1 AND success = $2 AND ip = $3)",
				tt.userID, tt.success, tt.ipAddress).Scan(&exists)
			require.NoError(t, err)
			require.True(t, exists)
		})
	}
}

func TestLoginAttemptRepository_GetRecentAttempts(t *testing.T) {
	tc := integration.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	// Create some login attempts with specific timestamps
	now := time.Now().UTC()
	timestamps := []time.Time{
		now,                        // Most recent
		now.Add(-30 * time.Minute), // Within last hour
		now.Add(-2 * time.Hour),    // Outside last hour
	}

	for _, ts := range timestamps {
		err := tc.LoginAttemptRepo.Create(context.Background(), user.ID, false, "127.0.0.1", ts)
		require.NoError(t, err)
	}

	tests := []struct {
		name      string
		userID    uuid.UUID
		since     time.Time
		wantCount int
		wantErr   error
	}{
		{
			name:      "All Recent Attempts",
			userID:    user.ID,
			since:     now.Add(-24 * time.Hour),
			wantCount: 3,
		},
		{
			name:      "Last Hour Only",
			userID:    user.ID,
			since:     now.Add(-1 * time.Hour),
			wantCount: 2,
		},
		{
			name:      "No Recent Attempts",
			userID:    user.ID,
			since:     now.Add(time.Hour),
			wantCount: 0,
		},
		{
			name:    "Non-existent User",
			userID:  uuid.New(),
			since:   now.Add(-24 * time.Hour),
			wantErr: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := tc.LoginAttemptRepo.GetRecentAttempts(context.Background(), tt.userID, tt.since)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantCount, count)
		})
	}
}

func TestLoginAttemptRepository_ClearAttempts(t *testing.T) {
	tc := integration.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	// Create some login attempts
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		err := tc.LoginAttemptRepo.Create(context.Background(), user.ID, false, "127.0.0.1", now)
		require.NoError(t, err)
	}

	tests := []struct {
		name    string
		userID  uuid.UUID
		wantErr error
	}{
		{
			name:   "Success",
			userID: user.ID,
		},
		{
			name:    "Non-existent User",
			userID:  uuid.New(),
			wantErr: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tc.LoginAttemptRepo.ClearAttempts(context.Background(), tt.userID)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			// Verify attempts were cleared
			count, err := tc.LoginAttemptRepo.GetRecentAttempts(context.Background(), tt.userID, now.Add(-24*time.Hour))
			require.NoError(t, err)
			require.Equal(t, 0, count)
		})
	}
}
