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

func TestRefreshTokenRepository_Create(t *testing.T) {
	tc := integration.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	tests := []struct {
		name    string
		userID  uuid.UUID
		wantErr bool
		errType error
	}{
		{
			name:    "Success",
			userID:  user.ID,
			wantErr: false,
		},
		{
			name:    "Non-existent User",
			userID:  uuid.New(),
			wantErr: true,
			errType: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := uuid.New().String()
			expiresAt := time.Now().UTC().Add(24 * time.Hour)

			err := tc.RefreshTokenRepo.Create(context.Background(), tt.userID, token, expiresAt)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)

				// Verify token was created
				var exists bool
				err = tc.DB.QueryRowContext(context.Background(),
					"SELECT EXISTS(SELECT 1 FROM refresh_tokens WHERE user_id = $1 AND token = $2)",
					tt.userID, token).Scan(&exists)
				require.NoError(t, err)
				require.True(t, exists)
			}
		})
	}
}

func TestRefreshTokenRepository_GetByToken(t *testing.T) {
	tc := integration.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	// Create a valid token
	validToken := uuid.New().String()
	validExpiresAt := time.Now().UTC().Add(24 * time.Hour)
	err := tc.RefreshTokenRepo.Create(context.Background(), user.ID, validToken, validExpiresAt)
	require.NoError(t, err)

	// Create an expired token
	expiredToken := uuid.New().String()
	expiredExpiresAt := time.Now().UTC().Add(-24 * time.Hour)
	err = tc.RefreshTokenRepo.Create(context.Background(), user.ID, expiredToken, expiredExpiresAt)
	require.NoError(t, err)

	tests := []struct {
		name    string
		token   string
		wantErr bool
		errType error
	}{
		{
			name:    "Success",
			token:   validToken,
			wantErr: false,
		},
		{
			name:    "Invalid Token",
			token:   "invalid-token",
			wantErr: true,
			errType: repository.ErrTokenInvalid,
		},
		{
			name:    "Expired Token",
			token:   expiredToken,
			wantErr: true,
			errType: repository.ErrTokenExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tc.RefreshTokenRepo.GetByToken(context.Background(), tt.token)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, user.ID, result.UserID)
				require.Equal(t, validToken, result.Token)
				require.Equal(t, validExpiresAt.Unix(), result.ExpiresAt.Unix())
			}
		})
	}
}

func TestRefreshTokenRepository_GetByUserID(t *testing.T) {
	tc := integration.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	// Create a token for the user
	token := uuid.New().String()
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	err := tc.RefreshTokenRepo.Create(context.Background(), user.ID, token, expiresAt)
	require.NoError(t, err)

	tests := []struct {
		name    string
		userID  uuid.UUID
		wantErr bool
		errType error
	}{
		{
			name:    "Success",
			userID:  user.ID,
			wantErr: false,
		},
		{
			name:    "Non-existent User",
			userID:  uuid.New(),
			wantErr: true,
			errType: repository.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := tc.RefreshTokenRepo.GetByUserID(context.Background(), tt.userID)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
				require.Len(t, results, 1)
				require.Equal(t, user.ID, results[0].UserID)
				require.Equal(t, token, results[0].Token)
				require.Equal(t, expiresAt.Unix(), results[0].ExpiresAt.Unix())
			}
		})
	}
}

func TestRefreshTokenRepository_Delete(t *testing.T) {
	tc := integration.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	// Create a token to delete
	token := uuid.New().String()
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	err := tc.RefreshTokenRepo.Create(context.Background(), user.ID, token, expiresAt)
	require.NoError(t, err)

	// Get the token ID
	var tokenID uuid.UUID
	err = tc.DB.QueryRowContext(context.Background(),
		"SELECT id FROM refresh_tokens WHERE token = $1",
		token).Scan(&tokenID)
	require.NoError(t, err)

	tests := []struct {
		name    string
		id      uuid.UUID
		wantErr bool
		errType error
	}{
		{
			name:    "Success",
			id:      tokenID,
			wantErr: false,
		},
		{
			name:    "Non-existent Token",
			id:      uuid.New(),
			wantErr: true,
			errType: repository.ErrTokenInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tc.RefreshTokenRepo.Delete(context.Background(), tt.id)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)

				// Verify token was deleted
				var exists bool
				err = tc.DB.QueryRowContext(context.Background(),
					"SELECT EXISTS(SELECT 1 FROM refresh_tokens WHERE id = $1)",
					tt.id).Scan(&exists)
				require.NoError(t, err)
				require.False(t, exists)
			}
		})
	}
}

func TestRefreshTokenRepository_DeleteByToken(t *testing.T) {
	tc := integration.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	// Create a token to delete
	token := uuid.New().String()
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	err := tc.RefreshTokenRepo.Create(context.Background(), user.ID, token, expiresAt)
	require.NoError(t, err)

	tests := []struct {
		name    string
		token   string
		wantErr bool
		errType error
	}{
		{
			name:    "Success",
			token:   token,
			wantErr: false,
		},
		{
			name:    "Non-existent Token",
			token:   "non-existent",
			wantErr: true,
			errType: repository.ErrTokenInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tc.RefreshTokenRepo.DeleteByToken(context.Background(), tt.token)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)

				// Verify token was deleted
				var exists bool
				err = tc.DB.QueryRowContext(context.Background(),
					"SELECT EXISTS(SELECT 1 FROM refresh_tokens WHERE token = $1)",
					tt.token).Scan(&exists)
				require.NoError(t, err)
				require.False(t, exists)
			}
		})
	}
}

func TestRefreshTokenRepository_DeleteByUserID(t *testing.T) {
	tc := integration.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	// Create multiple tokens for the user
	for i := 0; i < 3; i++ {
		token := uuid.New().String()
		expiresAt := time.Now().UTC().Add(24 * time.Hour)
		err := tc.RefreshTokenRepo.Create(context.Background(), user.ID, token, expiresAt)
		require.NoError(t, err)
	}

	err := tc.RefreshTokenRepo.DeleteByUserID(context.Background(), user.ID)
	require.NoError(t, err)

	// Verify all tokens were deleted
	var count int
	err = tc.DB.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1",
		user.ID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestRefreshTokenRepository_DeleteExpired(t *testing.T) {
	tc := integration.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	// Create expired tokens
	for i := 0; i < 3; i++ {
		token := uuid.New().String()
		expiresAt := time.Now().UTC().Add(-24 * time.Hour)
		err := tc.RefreshTokenRepo.Create(context.Background(), user.ID, token, expiresAt)
		require.NoError(t, err)
	}

	// Create valid token
	validToken := uuid.New().String()
	validExpiresAt := time.Now().UTC().Add(24 * time.Hour)
	err := tc.RefreshTokenRepo.Create(context.Background(), user.ID, validToken, validExpiresAt)
	require.NoError(t, err)

	err = tc.RefreshTokenRepo.DeleteExpired(context.Background())
	require.NoError(t, err)

	// Verify only expired tokens were deleted
	var count int
	err = tc.DB.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1",
		user.ID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestRefreshTokenRepository_IsValid(t *testing.T) {
	tc := integration.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	// Create a valid token
	validToken := uuid.New().String()
	validExpiresAt := time.Now().UTC().Add(24 * time.Hour)
	err := tc.RefreshTokenRepo.Create(context.Background(), user.ID, validToken, validExpiresAt)
	require.NoError(t, err)

	// Create an expired token
	expiredToken := uuid.New().String()
	expiredExpiresAt := time.Now().UTC().Add(-24 * time.Hour)
	err = tc.RefreshTokenRepo.Create(context.Background(), user.ID, expiredToken, expiredExpiresAt)
	require.NoError(t, err)

	tests := []struct {
		name    string
		token   string
		want    bool
		wantErr bool
		errType error
	}{
		{
			name:    "Valid Token",
			token:   validToken,
			want:    true,
			wantErr: false,
		},
		{
			name:    "Expired Token",
			token:   expiredToken,
			want:    false,
			wantErr: false,
		},
		{
			name:    "Non-existent Token",
			token:   "non-existent",
			want:    false,
			wantErr: true,
			errType: repository.ErrTokenInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := tc.RefreshTokenRepo.IsValid(context.Background(), tt.token)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, valid)
			}
		})
	}
}
