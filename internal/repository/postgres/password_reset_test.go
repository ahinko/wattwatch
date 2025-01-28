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

func TestPasswordResetRepository_Create(t *testing.T) {
	tc := integration.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	tests := []struct {
		name    string
		userID  uuid.UUID
		wantErr error
	}{
		{
			name:    "Success - Create Reset Token",
			userID:  user.ID,
			wantErr: nil,
		},
		{
			name:    "Error - Non-existent User",
			userID:  uuid.New(),
			wantErr: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reset, err := tc.PasswordResetRepo.Create(context.Background(), tt.userID)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, reset)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, reset)
			require.NotEqual(t, uuid.Nil, reset.ID)
			require.Equal(t, tt.userID, reset.UserID)
			require.NotEmpty(t, reset.Token)
			require.False(t, reset.ExpiresAt.IsZero())
			require.False(t, reset.CreatedAt.IsZero())
			require.True(t, reset.ExpiresAt.After(time.Now()))
			require.Nil(t, reset.UsedAt)

			// Verify reset was created in database
			var exists bool
			err = tc.DB.QueryRowContext(context.Background(),
				"SELECT EXISTS(SELECT 1 FROM password_resets WHERE id = $1 AND user_id = $2 AND token = $3 AND used_at IS NULL)",
				reset.ID, reset.UserID, reset.Token).Scan(&exists)
			require.NoError(t, err)
			require.True(t, exists, "Password reset record should exist in database")
		})
	}
}

func TestPasswordResetRepository_GetByToken(t *testing.T) {
	tc := integration.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	// Create a valid reset token
	validReset, err := tc.PasswordResetRepo.Create(context.Background(), user.ID)
	require.NoError(t, err)

	// Create an expired token
	expiredReset, err := tc.PasswordResetRepo.Create(context.Background(), user.ID)
	require.NoError(t, err)
	_, err = tc.DB.ExecContext(context.Background(),
		"UPDATE password_resets SET expires_at = $1 WHERE id = $2",
		time.Now().Add(-24*time.Hour), expiredReset.ID)
	require.NoError(t, err)

	// Create a used token
	usedReset, err := tc.PasswordResetRepo.Create(context.Background(), user.ID)
	require.NoError(t, err)
	err = tc.PasswordResetRepo.MarkAsUsed(context.Background(), usedReset.ID)
	require.NoError(t, err)

	tests := []struct {
		name    string
		token   string
		wantErr error
	}{
		{
			name:    "Success - Valid Token",
			token:   validReset.Token,
			wantErr: nil,
		},
		{
			name:    "Error - Invalid Token Format",
			token:   "invalid-token",
			wantErr: repository.ErrResetTokenInvalid,
		},
		{
			name:    "Error - Expired Token",
			token:   expiredReset.Token,
			wantErr: repository.ErrResetTokenExpired,
		},
		{
			name:    "Error - Used Token",
			token:   usedReset.Token,
			wantErr: repository.ErrResetTokenUsed,
		},
		{
			name:    "Error - Empty Token",
			token:   "",
			wantErr: repository.ErrResetTokenInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tc.PasswordResetRepo.GetByToken(context.Background(), tt.token)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, result)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, validReset.ID, result.ID)
			require.Equal(t, validReset.UserID, result.UserID)
			require.Equal(t, validReset.Token, result.Token)
			require.Equal(t, validReset.ExpiresAt.Unix(), result.ExpiresAt.Unix())
			require.Nil(t, result.UsedAt)
			require.False(t, result.CreatedAt.IsZero())
			require.True(t, result.ExpiresAt.After(time.Now()))
		})
	}
}

func TestPasswordResetRepository_MarkAsUsed(t *testing.T) {
	tc := integration.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	// Create a valid reset token
	validReset, err := tc.PasswordResetRepo.Create(context.Background(), user.ID)
	require.NoError(t, err)

	// Create and mark a token as used
	usedReset, err := tc.PasswordResetRepo.Create(context.Background(), user.ID)
	require.NoError(t, err)
	err = tc.PasswordResetRepo.MarkAsUsed(context.Background(), usedReset.ID)
	require.NoError(t, err)

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr error
	}{
		{
			name:    "Success - Mark Valid Token as Used",
			id:      validReset.ID,
			wantErr: nil,
		},
		{
			name:    "Error - Non-existent Token",
			id:      uuid.New(),
			wantErr: repository.ErrResetTokenInvalid,
		},
		{
			name:    "Error - Already Used Token",
			id:      usedReset.ID,
			wantErr: repository.ErrResetTokenInvalid,
		},
		{
			name:    "Error - Zero UUID",
			id:      uuid.Nil,
			wantErr: repository.ErrResetTokenInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tc.PasswordResetRepo.MarkAsUsed(context.Background(), tt.id)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)

			// Verify token was marked as used
			var usedAt *time.Time
			err = tc.DB.QueryRowContext(context.Background(),
				"SELECT used_at FROM password_resets WHERE id = $1",
				tt.id).Scan(&usedAt)
			require.NoError(t, err)
			require.NotNil(t, usedAt)
			require.WithinDuration(t, time.Now(), *usedAt, 2*time.Second)

			// Verify we can't use the token again
			err = tc.PasswordResetRepo.MarkAsUsed(context.Background(), tt.id)
			require.ErrorIs(t, err, repository.ErrResetTokenInvalid)
		})
	}
}
