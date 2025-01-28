// Package integration provides utilities for postgres integration testing
package integration

import (
	"context"
	"testing"
	"time"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"
	"wattwatch/internal/repository/postgres"
	"wattwatch/internal/testutil"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestContext wraps testutil.TestContext to provide postgres-specific test utilities
type TestContext struct {
	*testutil.TestContext
	SpotPriceRepo repository.SpotPriceRepository
}

// NewTestContext creates a new test context for postgres integration tests
func NewTestContext(t *testing.T) *TestContext {
	tc := testutil.NewTestContext(t)
	return &TestContext{
		TestContext:   tc,
		SpotPriceRepo: postgres.NewSpotPriceRepository(tc.DB),
	}
}

// CleanupTestUsers removes all test users except admin
func (tc *TestContext) CleanupTestUsers() {
	tc.T.Helper()
	tc.ExecuteSQL("DELETE FROM users WHERE username != 'admin'")
}

// CleanupTestRoles removes all test roles except admin and user
func (tc *TestContext) CleanupTestRoles() {
	tc.T.Helper()
	tc.ExecuteSQL("DELETE FROM roles WHERE name != 'admin' AND name != 'user'")
}

// CleanupTestCurrencies removes all test currencies
func (tc *TestContext) CleanupTestCurrencies() {
	tc.T.Helper()
	tc.ExecuteSQL("DELETE FROM currencies")
}

// CleanupTestZones removes all test zones
func (tc *TestContext) CleanupTestZones() {
	tc.T.Helper()
	tc.ExecuteSQL("DELETE FROM zones")
}

// CreateTestLoginAttempt creates a test login attempt
func (tc *TestContext) CreateTestLoginAttempt(userID uuid.UUID, success bool, ip string) *models.LoginAttempt {
	tc.T.Helper()
	attempt := &models.LoginAttempt{
		UserID:  userID,
		Success: success,
		IP:      ip,
	}
	err := tc.LoginAttemptRepo.Create(context.Background(), attempt.UserID, attempt.Success, attempt.IP, attempt.CreatedAt)
	require.NoError(tc.T, err)
	return attempt
}

// CreateTestPasswordReset creates a test password reset token
func (tc *TestContext) CreateTestPasswordReset(userID uuid.UUID) *repository.PasswordReset {
	tc.T.Helper()
	reset, err := tc.PasswordResetRepo.Create(context.Background(), userID)
	require.NoError(tc.T, err)
	return reset
}

// CreateTestEmailVerification creates a test email verification token
func (tc *TestContext) CreateTestEmailVerification(userID uuid.UUID) *repository.EmailVerification {
	tc.T.Helper()
	verify, err := tc.EmailVerifyRepo.Create(context.Background(), userID)
	require.NoError(tc.T, err)
	return verify
}

// CreateTestSpotPrice creates a test spot price
func (tc *TestContext) CreateTestSpotPrice(zoneID, currencyID uuid.UUID, price float64, timestamp time.Time) *models.SpotPrice {
	tc.T.Helper()
	spotPrice := &models.SpotPrice{
		ZoneID:     zoneID,
		CurrencyID: currencyID,
		Price:      price,
		Timestamp:  timestamp,
	}
	err := tc.CurrencyRepo.Create(context.Background(), &models.Currency{ID: currencyID})
	require.NoError(tc.T, err)
	err = tc.ZoneRepo.Create(context.Background(), &models.Zone{ID: zoneID})
	require.NoError(tc.T, err)
	return spotPrice
}

// CreateTestAuditLog creates a test audit log entry
func (tc *TestContext) CreateTestAuditLog(userID *uuid.UUID, action models.AuditAction, entityType, entityID, description string) *models.AuditLog {
	tc.T.Helper()
	req := &models.CreateAuditLogRequest{
		UserID:      userID,
		Action:      action,
		EntityType:  entityType,
		EntityID:    entityID,
		Description: description,
		IPAddress:   "127.0.0.1",
		UserAgent:   "test-agent",
	}
	err := tc.AuditRepo.Create(context.Background(), req)
	require.NoError(tc.T, err)
	return nil // TODO: Return actual audit log once we add GetByID
}

// ExecuteSQL executes a raw SQL query for testing
func (tc *TestContext) ExecuteSQL(query string, args ...interface{}) {
	tc.T.Helper()
	_, err := tc.DB.ExecContext(context.Background(), query, args...)
	require.NoError(tc.T, err)
}
