// Package testutil provides utilities for testing
package testutil

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"wattwatch/internal/api/handlers"
	"wattwatch/internal/auth"
	"wattwatch/internal/config"
	"wattwatch/internal/email"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"
	"wattwatch/internal/repository/postgres"
	"wattwatch/internal/testutil/db"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// LoadTestConfig loads the test configuration
func LoadTestConfig(t *testing.T) *config.Config {
	t.Helper()
	return db.LoadTestConfig(t)
}

// TestContext holds common test dependencies
type TestContext struct {
	T                   *testing.T
	DB                  *sql.DB
	Config              *config.Config
	UserRepo            repository.UserRepository
	RoleRepo            repository.RoleRepository
	PasswordHistoryRepo repository.PasswordHistoryRepository
	EmailVerifyRepo     repository.EmailVerificationRepository
	PasswordResetRepo   repository.PasswordResetRepository
	LoginAttemptRepo    repository.LoginAttemptRepository
	AuditRepo           repository.AuditLogRepository
	AuthService         *auth.Service
	EmailService        email.EmailSender
	AuthHandler         *handlers.AuthHandler
	RefreshTokenRepo    repository.RefreshTokenRepository
	ZoneRepo            repository.ZoneRepository
	CurrencyRepo        repository.CurrencyRepository
}

// MockEmailService is a mock implementation of the email service for testing
type MockEmailService struct{}

func NewMockEmailService() *MockEmailService {
	return &MockEmailService{}
}

func (s *MockEmailService) SendVerificationEmail(to, username, token string) error {
	return nil
}

func (s *MockEmailService) SendPasswordResetEmail(to, username, token string) error {
	return nil
}

// NewTestContext creates a new test context with all dependencies
func NewTestContext(t *testing.T) *TestContext {
	t.Helper()

	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Initialize validators
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		err := v.RegisterValidation("nospaces", func(fl validator.FieldLevel) bool {
			value := fl.Field().String()
			return strings.TrimSpace(value) != ""
		})
		if err != nil {
			t.Fatal("Failed to register validator:", err)
		}
	}

	// Load test config
	cfg := LoadTestConfig(t)

	// Setup test database
	testDB := db.SetupTestDB(t, &cfg.Database)

	// Initialize repositories
	userRepo := postgres.NewUserRepository(testDB)
	roleRepo := postgres.NewRoleRepository(testDB)
	passwordHistoryRepo := postgres.NewPasswordHistoryRepository(testDB)
	emailVerifyRepo := postgres.NewEmailVerificationRepository(testDB)
	passwordResetRepo := postgres.NewPasswordResetRepository(testDB)
	loginAttemptRepo := postgres.NewLoginAttemptRepository(testDB)
	auditRepo := postgres.NewAuditLogRepository(testDB)
	refreshTokenRepo := postgres.NewRefreshTokenRepository(testDB)
	zoneRepo := postgres.NewZoneRepository(testDB)
	currencyRepo := postgres.NewCurrencyRepository(testDB)

	// Initialize services
	authService := auth.NewService(cfg, refreshTokenRepo)
	emailService := &MockEmailService{} // Use mock email service for testing

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(
		userRepo,
		roleRepo,
		authService,
		auditRepo,
		emailService,
		cfg,
		loginAttemptRepo,
		emailVerifyRepo,
		passwordResetRepo,
	)

	tc := &TestContext{
		T:                   t,
		DB:                  testDB,
		Config:              cfg,
		UserRepo:            userRepo,
		RoleRepo:            roleRepo,
		PasswordHistoryRepo: passwordHistoryRepo,
		EmailVerifyRepo:     emailVerifyRepo,
		PasswordResetRepo:   passwordResetRepo,
		LoginAttemptRepo:    loginAttemptRepo,
		AuditRepo:           auditRepo,
		RefreshTokenRepo:    refreshTokenRepo,
		AuthService:         authService,
		EmailService:        emailService,
		AuthHandler:         authHandler,
		ZoneRepo:            zoneRepo,
		CurrencyRepo:        currencyRepo,
	}

	// Register cleanup function
	t.Cleanup(func() {
		tc.cleanup()
	})

	return tc
}

// cleanup performs necessary cleanup after tests
func (tc *TestContext) cleanup() {
	if tc.DB != nil {
		if err := db.CleanupTestDB(tc.DB); err != nil {
			tc.T.Errorf("Failed to cleanup test database: %v", err)
		}
		tc.DB.Close()
	}
}

// CreateTestUser creates a test user with the given details and returns the created user
func (tc *TestContext) CreateTestUser(username, email, password string, isAdmin bool) *models.User {
	tc.T.Helper()

	// Get the appropriate role
	var role *models.Role
	var err error
	if isAdmin {
		role, err = tc.RoleRepo.GetByName(context.Background(), "admin")
		require.NoError(tc.T, err, "Failed to get admin role")
	} else {
		role, err = tc.RoleRepo.GetByName(context.Background(), "user")
		require.NoError(tc.T, err, "Failed to get user role")
	}

	// Create the user
	emailPtr := &email
	user := &models.User{
		Username: username,
		Password: password,
		Email:    emailPtr,
		RoleID:   role.ID,
		Role:     role,
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(tc.T, err, "Failed to hash password")
	user.Password = string(hashedPassword)

	// Save the user
	err = tc.UserRepo.Create(context.Background(), user)
	require.NoError(tc.T, err, "Failed to create test user")

	return user
}

// CreateTestUserFromRequest creates a test user from a CreateUserRequest
func (tc *TestContext) CreateTestUserFromRequest(req models.CreateUserRequest, isAdmin bool) *models.User {
	email := ""
	if req.Email != nil {
		email = *req.Email
	}
	return tc.CreateTestUser(req.Username, email, req.Password, isAdmin)
}

// GetTestJWT generates a JWT token for testing
func (tc *TestContext) GetTestJWT(userID uuid.UUID) string {
	user, err := tc.UserRepo.GetByID(context.Background(), userID)
	require.NoError(tc.T, err, "Failed to get user")

	// Get user's role to check if they're admin
	role, err := tc.RoleRepo.GetByID(context.Background(), user.RoleID)
	require.NoError(tc.T, err, "Failed to get user's role")

	// Set the role on the user
	user.Role = role

	token, err := tc.AuthService.GenerateToken(user, false)
	require.NoError(tc.T, err, "Failed to generate test JWT")
	return token
}

// CreateTestRole creates a test role with the given name and returns it
func (tc *TestContext) CreateTestRole(name string, isProtected bool, isAdminGroup bool) *models.Role {
	tc.T.Helper()

	role := &models.Role{
		Name:         name,
		IsProtected:  isProtected,
		IsAdminGroup: isAdminGroup,
	}

	err := tc.RoleRepo.Create(context.Background(), role)
	require.NoError(tc.T, err, "Failed to create test role")

	return role
}

// GetRoleID returns a pointer to the role ID for the given role name
func (tc *TestContext) GetRoleID(roleName string) *uuid.UUID {
	tc.T.Helper()
	role, err := tc.RoleRepo.GetByName(context.Background(), roleName)
	require.NoError(tc.T, err, "Failed to get role")
	return &role.ID
}

// MarkEmailVerified marks a user's email as verified
func (tc *TestContext) MarkEmailVerified(userID uuid.UUID) {
	tc.T.Helper()
	err := tc.UserRepo.VerifyEmail(context.Background(), userID)
	require.NoError(tc.T, err, "Failed to mark email as verified")
}

// CreateTestZone creates a test zone with the given name and timezone and returns it
func (tc *TestContext) CreateTestZone(name, timezone string) *models.Zone {
	tc.T.Helper()

	zone := &models.Zone{
		Name:     name,
		Timezone: timezone,
	}

	err := tc.ZoneRepo.Create(context.Background(), zone)
	require.NoError(tc.T, err, "Failed to create test zone")

	return zone
}

// CreateTestCurrency creates a test currency with the given name and returns it
func (tc *TestContext) CreateTestCurrency(name string) *models.Currency {
	tc.T.Helper()

	currency := &models.Currency{
		Name: name,
	}

	err := tc.CurrencyRepo.Create(context.Background(), currency)
	require.NoError(tc.T, err, "Failed to create test currency")

	return currency
}
