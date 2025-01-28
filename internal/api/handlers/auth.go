package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"wattwatch/internal/auth"
	"wattwatch/internal/config"
	"wattwatch/internal/email"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"

	"github.com/gin-gonic/gin"
)

// AuthHandler handles HTTP requests for authentication and user management
type AuthHandler struct {
	userRepo          repository.UserRepository
	roleRepo          repository.RoleRepository
	authService       *auth.Service
	auditRepo         repository.AuditLogRepository
	emailService      email.EmailSender
	config            *config.Config
	loginAttemptRepo  repository.LoginAttemptRepository
	emailVerifyRepo   repository.EmailVerificationRepository
	passwordResetRepo repository.PasswordResetRepository
}

// NewAuthHandler creates a new authentication handler with the given dependencies
func NewAuthHandler(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	authService *auth.Service,
	auditRepo repository.AuditLogRepository,
	emailService email.EmailSender,
	config *config.Config,
	loginAttemptRepo repository.LoginAttemptRepository,
	emailVerifyRepo repository.EmailVerificationRepository,
	passwordResetRepo repository.PasswordResetRepository,
) *AuthHandler {
	return &AuthHandler{
		userRepo:          userRepo,
		roleRepo:          roleRepo,
		authService:       authService,
		auditRepo:         auditRepo,
		emailService:      emailService,
		config:            config,
		loginAttemptRepo:  loginAttemptRepo,
		emailVerifyRepo:   emailVerifyRepo,
		passwordResetRepo: passwordResetRepo,
	}
}

// LoginRequest represents the login credentials
type LoginRequest struct {
	Username string `json:"username" binding:"required,max=50" example:"johndoe"`
	Password string `json:"password" binding:"required" example:"mypassword123"`
}

// LoginResponse represents the response after successful login
type LoginResponse struct {
	AccessToken  string `json:"access_token" example:"eyJhbGciOiJIUzI1NiIs..."`
	RefreshToken string `json:"refresh_token" example:"dG9rZW4uLi4="`
}

// Login godoc
// @Summary User login
// @Description Authenticate user and return access and refresh tokens
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "Login credentials"
// @Success 200 {object} models.LoginResponse "Login successful"
// @Failure 400 {object} models.ErrorResponse "Invalid request format"
// @Failure 401 {object} models.ErrorResponse "Invalid credentials"
// @Failure 403 {object} models.ErrorResponse "Account locked or email not verified"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	ipAddress := c.ClientIP()

	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Validate username length
	if len(req.Username) > 50 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Key: 'LoginRequest.Username' Error:Field validation for 'Username' failed on the 'max' tag"})
		return
	}

	// Get user first to check if exists and is active
	user, err := h.userRepo.GetByUsername(c.Request.Context(), req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid credentials"})
		return
	}

	// Check if account is active before anything else
	if user.DeletedAt != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "account is inactive"})
		return
	}

	// Check for too many recent failed attempts
	cutoff := time.Now().Add(-15 * time.Minute)
	recentAttempts, err := h.loginAttemptRepo.GetRecentAttempts(c.Request.Context(), user.ID, cutoff)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to process login"})
		return
	}

	if recentAttempts >= repository.MaxLoginAttempts {
		c.JSON(http.StatusTooManyRequests, models.ErrorResponse{Error: "too many failed login attempts"})
		return
	}

	// Verify password before recording attempt
	if err := h.authService.ComparePasswords(user.Password, req.Password); err != nil {
		// Record failed attempt
		if err := h.loginAttemptRepo.Create(c.Request.Context(), user.ID, false, ipAddress, time.Now()); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to process login"})
			return
		}
		if err := h.userRepo.IncrementFailedAttempts(c.Request.Context(), req.Username); err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to process login"})
			return
		}
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid credentials"})
		return
	}

	// Record successful attempt
	if err := h.loginAttemptRepo.Create(c.Request.Context(), user.ID, true, ipAddress, time.Now()); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to process login"})
		return
	}

	// Reset failed attempts on successful login
	if err := h.userRepo.ResetFailedAttempts(c.Request.Context(), req.Username); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to process login"})
		return
	}

	// Clear login attempts
	if err := h.loginAttemptRepo.ClearAttempts(c.Request.Context(), user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to process login"})
		return
	}

	// Update last login
	if err := h.userRepo.UpdateLastLogin(c.Request.Context(), user.ID, time.Now()); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update login time"})
		return
	}

	// Create audit log entry for successful login
	details, _ := json.Marshal(map[string]interface{}{"username": user.Username})
	auditLog := &models.CreateAuditLogRequest{
		UserID:      &user.ID,
		Action:      "login_success",
		EntityType:  "user",
		EntityID:    user.ID.String(),
		Description: fmt.Sprintf("User %s logged in successfully", user.Username),
		Metadata:    string(details),
		IPAddress:   ipAddress,
		UserAgent:   c.GetHeader("User-Agent"),
	}
	if err := h.auditRepo.Create(c.Request.Context(), auditLog); err != nil {
		// Log error but don't fail the login
		log.Printf("Failed to create audit log: %v", err)
	}

	role, err := h.roleRepo.GetByID(c.Request.Context(), user.RoleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get user role"})
		return
	}
	user.Role = role

	// Generate access token
	accessToken, err := h.authService.GenerateToken(user, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate access token"})
		return
	}

	// Generate refresh token
	refreshToken, err := h.authService.GenerateRefreshToken(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate refresh token"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// Register godoc
// @Summary Register new user
// @Description Register a new user account. First user gets admin role, subsequent users get user role.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.CreateUserRequest true "User registration details"
// @Success 201 {object} models.User "User created successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid request format, username/email already exists, or validation error"
// @Failure 403 {object} models.ErrorResponse "Registration is disabled (unless admin or first user)"
// @Failure 409 {object} models.ErrorResponse "Username or email already exists"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Failed to create user or process request"
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Get user from context if authenticated
	isAdmin := c.GetBool("is_admin")

	// Get existing users count
	users, err := h.userRepo.List(c.Request.Context(), repository.UserFilter{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to check existing users"})
		return
	}

	// Allow registration if:
	// 1. No users exist (first user)
	// 2. Registration is open
	// 3. User is an admin
	isFirstUser := len(users) == 0
	if !isFirstUser && !isAdmin && !h.config.Auth.RegistrationOpen {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "registration is disabled"})
		return
	}

	// Check if username exists
	existingUser, err := h.userRepo.GetByUsername(c.Request.Context(), req.Username)
	if err != nil && err != repository.ErrUserNotFound {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to check username"})
		return
	}
	if existingUser != nil {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "username already exists"})
		return
	}

	// Check if email exists if provided
	if req.Email != nil {
		existingUser, err = h.userRepo.GetByEmail(c.Request.Context(), *req.Email)
		if err != nil && err != repository.ErrUserNotFound {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to check email"})
			return
		}
		if existingUser != nil {
			c.JSON(http.StatusConflict, models.ErrorResponse{Error: "email already exists"})
			return
		}
	}

	// Hash password
	hashedPassword, err := h.authService.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to process registration"})
		return
	}

	// Get appropriate role
	var role *models.Role
	if isFirstUser {
		role, err = h.roleRepo.GetByName(c.Request.Context(), "admin")
	} else {
		role, err = h.roleRepo.GetByName(c.Request.Context(), "user")
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get role"})
		return
	}

	// Create user
	user := &models.User{
		Username:      req.Username,
		Password:      hashedPassword,
		Email:         req.Email,
		RoleID:        role.ID,
		Role:          role,
		EmailVerified: false,
	}

	if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create user"})
		return
	}

	// Send verification email if email provided
	if req.Email != nil {
		verification, err := h.emailVerifyRepo.Create(c.Request.Context(), user.ID)
		if err != nil {
			// Don't fail registration if email verification fails
			log.Printf("Failed to create email verification: %v", err)
		} else {
			if err := h.emailService.SendVerificationEmail(*req.Email, req.Username, verification.Token); err != nil {
				// Don't fail registration if sending email fails
				log.Printf("Failed to send verification email: %v", err)
			}
		}
	}

	// Create audit log
	details, _ := json.Marshal(map[string]interface{}{
		"username": user.Username,
		"role":     role.Name,
	})
	auditLog := &models.CreateAuditLogRequest{
		UserID:      &user.ID,
		Action:      "user_registered",
		EntityType:  "user",
		EntityID:    user.ID.String(),
		Description: fmt.Sprintf("User %s registered successfully", user.Username),
		Metadata:    string(details),
		IPAddress:   c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
	}
	if err := h.auditRepo.Create(c.Request.Context(), auditLog); err != nil {
		// Don't fail registration if audit log fails
		log.Printf("Failed to create audit log: %v", err)
	}

	c.JSON(http.StatusCreated, user)
}

// VerifyEmail godoc
// @Summary Verify email address
// @Description Verify a user's email address using the verification token
// @Tags auth
// @Accept json
// @Produce json
// @Param token query string true "Email verification token"
// @Success 200 {object} models.SuccessResponse "Email verified successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid, expired, or missing token"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /auth/verify-email [get]
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "verification token is required"})
		return
	}

	// Verify email
	if err := h.emailVerifyRepo.Verify(c.Request.Context(), token); err != nil {
		switch err {
		case repository.ErrTokenExpired:
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "verification token has expired"})
		case repository.ErrTokenInvalid:
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid verification token"})
		default:
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to verify email"})
		}
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "email verified successfully"})
}

// ResendVerification godoc
// @Summary Resend verification email
// @Description Resend verification email for authenticated user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.ResendVerificationRequest true "Resend verification request"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse "Email already verified or missing"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /auth/resend-verification [post]
func (h *AuthHandler) ResendVerification(c *gin.Context) {
	var req models.ResendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Get user by email
	user, err := h.userRepo.GetByEmail(c.Request.Context(), req.Email)
	if err == repository.ErrUserNotFound {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "no user found with this email address"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get user"})
		return
	}

	// Check if email exists
	if user.Email == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "no email address associated with account"})
		return
	}

	// Check if already verified
	if user.EmailVerified {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "email already verified"})
		return
	}

	// Create verification token
	verification, err := h.emailVerifyRepo.Create(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create verification token"})
		return
	}

	// Send verification email
	err = h.emailService.SendVerificationEmail(req.Email, user.Username, verification.Token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to send verification email"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "verification email sent"})
}

// RequestPasswordReset godoc
// @Summary Request password reset
// @Description Request a password reset email. For security, always returns success even if email doesn't exist.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.PasswordResetRequest true "User's email"
// @Success 200 {object} models.SuccessResponse "Reset link will be sent if email exists"
// @Failure 400 {object} models.ErrorResponse "Invalid email format or user has no email"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Failed to process request, create token, or send email"
// @Router /auth/reset-password [post]
func (h *AuthHandler) RequestPasswordReset(c *gin.Context) {
	var req models.PasswordResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Find user by email
	user, err := h.userRepo.GetByEmail(c.Request.Context(), req.Email)
	if err == repository.ErrUserNotFound {
		// Return success even if email doesn't exist (security)
		c.JSON(http.StatusOK, models.SuccessResponse{Message: "if the email exists, a reset link will be sent"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to process request"})
		return
	}

	// Check if user has an email
	if user.Email == nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "user has no email address"})
		return
	}

	// Check if email is verified
	if !user.EmailVerified {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "email address must be verified before requesting a password reset"})
		return
	}

	// Create password reset token
	reset, err := h.passwordResetRepo.Create(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create reset token"})
		return
	}

	// Send password reset email
	err = h.emailService.SendPasswordResetEmail(*user.Email, user.Username, reset.Token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to send password reset email"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "if the email exists, a reset link will be sent"})
}

// CompletePasswordReset godoc
// @Summary Complete password reset
// @Description Reset user's password using reset token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.CompleteResetRequest true "Reset completion details"
// @Success 200 {object} models.SuccessResponse "Password reset successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid request, expired/invalid/used token, or password reuse"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Failed to verify token, process password, or update user"
// @Router /auth/reset-password/complete [post]
func (h *AuthHandler) CompletePasswordReset(c *gin.Context) {
	var req models.CompleteResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Verify token
	reset, err := h.passwordResetRepo.GetByToken(c.Request.Context(), req.Token)
	switch err {
	case nil:
		// Token is valid
	case repository.ErrResetTokenExpired:
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "reset token has expired"})
		return
	case repository.ErrResetTokenInvalid:
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid reset token"})
		return
	case repository.ErrResetTokenUsed:
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "reset token has already been used"})
		return
	default:
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to verify token"})
		return
	}

	// Hash new password
	hashedPassword, err := h.authService.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to process password"})
		return
	}

	// Update password
	if err := h.userRepo.UpdatePassword(c.Request.Context(), reset.UserID, hashedPassword); err != nil {
		if err == repository.ErrPasswordReuse {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "cannot reuse recent passwords"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update password"})
		return
	}

	// Mark reset token as used
	if err := h.passwordResetRepo.MarkAsUsed(c.Request.Context(), reset.ID); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to complete reset"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "password reset successfully"})
}

// RefreshRequest represents the request to refresh an access token
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required" example:"dG9rZW4uLi4="`
}

// RefreshResponse represents the response after refreshing an access token
type RefreshResponse struct {
	AccessToken string `json:"access_token" example:"eyJhbGciOiJIUzI1NiIs..."`
}

// Refresh godoc
// @Summary Refresh access token
// @Description Get a new access token using a refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "Refresh token"
// @Success 200 {object} RefreshResponse
// @Failure 400 {object} models.ErrorResponse "Invalid request"
// @Failure 401 {object} models.ErrorResponse "Invalid or expired refresh token"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Validate refresh token
	userID, err := h.authService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		// Any error validating the token should result in 401
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid or expired refresh token"})
		return
	}

	// Get user
	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get user"})
		return
	}

	// Load user's role
	role, err := h.roleRepo.GetByID(c.Request.Context(), user.RoleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get user role"})
		return
	}
	user.Role = role

	// Generate new access token
	accessToken, err := h.authService.GenerateToken(user, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to generate access token"})
		return
	}

	c.JSON(http.StatusOK, RefreshResponse{
		AccessToken: accessToken,
	})
}
