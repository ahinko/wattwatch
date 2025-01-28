package models

// TokenRefreshRequest represents a token refresh request
type TokenRefreshRequest struct {
	Token string `json:"token" binding:"required"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username" binding:"required,max=50"`
	Password string `json:"password" binding:"required"`
}

// PasswordResetRequest represents a password reset request
type PasswordResetRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// EmailVerificationRequest represents an email verification request
type EmailVerificationRequest struct {
	Token string `json:"token" binding:"required"`
}

// ResendVerificationRequest represents a request to resend verification email
type ResendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}
