package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system
type User struct {
	ID                  uuid.UUID  `json:"id"`
	Username            string     `json:"username"`
	Password            string     `json:"-"`
	Email               *string    `json:"email"`
	EmailVerified       bool       `json:"email_verified"`
	RoleID              uuid.UUID  `json:"role_id"`
	Role                *Role      `json:"role,omitempty"`
	LastLoginAt         *time.Time `json:"last_login_at"`
	LastFailedLogin     *time.Time `json:"last_failed_login,omitempty"`
	PasswordChangedAt   *time.Time `json:"password_changed_at,omitempty"`
	FailedLoginAttempts int        `json:"-"`
	DeletedAt           *time.Time `json:"deleted_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// CreateUserRequest represents the request to create a new user
type CreateUserRequest struct {
	Username string  `json:"username" binding:"required,min=3,max=50" validate:"max=50"`
	Password string  `json:"password" binding:"required,min=8"`
	Email    *string `json:"email" binding:"omitempty,email"`
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
	Email    *string    `json:"email,omitempty" binding:"omitempty,email"`
	Password *string    `json:"password,omitempty" binding:"omitempty,min=8"`
	RoleID   *uuid.UUID `json:"role_id,omitempty"`
}

// ChangePasswordRequest represents the request to change a user's password
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8,max=72"`
}

// ResetPasswordRequest represents the request to initiate a password reset
type ResetPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// CompleteResetRequest represents the request to complete a password reset
type CompleteResetRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=72"`
}

// IsAdmin returns true if the user has an admin role
func (u *User) IsAdmin() bool {
	return u.Role != nil && u.Role.IsAdminGroup
}
