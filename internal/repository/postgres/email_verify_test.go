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

func TestEmailVerificationRepository_Create(t *testing.T) {
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
			verify, err := tc.EmailVerifyRepo.Create(context.Background(), tt.userID)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, verify)
				require.NotEqual(t, uuid.Nil, verify.ID)
				require.Equal(t, tt.userID, verify.UserID)
				require.NotEmpty(t, verify.Token)
				require.False(t, verify.ExpiresAt.IsZero())
				require.Nil(t, verify.VerifiedAt)
			}
		})
	}
}

func TestEmailVerificationRepository_Verify(t *testing.T) {
	tc := integration.NewTestContext(t)
	user := tc.CreateTestUser("test-user", "test@example.com", "password123", false)

	// Create a valid verification token
	verify, err := tc.EmailVerifyRepo.Create(context.Background(), user.ID)
	require.NoError(t, err)

	// Create an expired token
	expiredVerify, err := tc.EmailVerifyRepo.Create(context.Background(), user.ID)
	require.NoError(t, err)
	_, err = tc.DB.ExecContext(context.Background(),
		"UPDATE email_verifications SET expires_at = $1 WHERE id = $2",
		time.Now().Add(-24*time.Hour), expiredVerify.ID)
	require.NoError(t, err)

	// Create a used token
	usedVerify, err := tc.EmailVerifyRepo.Create(context.Background(), user.ID)
	require.NoError(t, err)
	err = tc.EmailVerifyRepo.Verify(context.Background(), usedVerify.Token)
	require.NoError(t, err)

	tests := []struct {
		name    string
		token   string
		wantErr bool
		errType error
	}{
		{
			name:    "Success",
			token:   verify.Token,
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
			token:   expiredVerify.Token,
			wantErr: true,
			errType: repository.ErrTokenExpired,
		},
		{
			name:    "Used Token",
			token:   usedVerify.Token,
			wantErr: true,
			errType: repository.ErrTokenInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tc.EmailVerifyRepo.Verify(context.Background(), tt.token)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
			} else {
				require.NoError(t, err)

				// Verify token was marked as verified
				var verifiedAt *time.Time
				err = tc.DB.QueryRowContext(context.Background(),
					"SELECT verified_at FROM email_verifications WHERE token = $1",
					tt.token).Scan(&verifiedAt)
				require.NoError(t, err)
				require.NotNil(t, verifiedAt)
				require.WithinDuration(t, time.Now(), *verifiedAt, 2*time.Second)

				// Verify user's email was marked as verified
				var emailVerified bool
				err = tc.DB.QueryRowContext(context.Background(),
					"SELECT email_verified FROM users WHERE id = $1",
					user.ID).Scan(&emailVerified)
				require.NoError(t, err)
				require.True(t, emailVerified)
			}
		})
	}
}
