package repository

import "errors"

var (
	// Common errors
	ErrNotFound             = errors.New("not found")
	ErrConflict             = errors.New("conflict")
	ErrHasAssociatedRecords = errors.New("has associated records")
	ErrInvalidTimezone      = errors.New("invalid timezone")
	ErrDuplicateEntry       = errors.New("duplicate entry")

	// User errors
	ErrEmailExists     = errors.New("email already exists")
	ErrUsernameExists  = errors.New("username already exists")
	ErrUserProtected   = errors.New("user is protected")
	ErrInvalidPassword = errors.New("invalid password")
	ErrUserLocked      = errors.New("user is locked")
	ErrUserNotFound    = errors.New("user not found")
	ErrUserExists      = errors.New("user already exists")
	ErrAdminDelete     = errors.New("cannot delete admin user")

	// Role errors
	ErrRoleProtected = errors.New("role is protected")
	ErrRoleExists    = errors.New("role already exists")
	ErrRoleInUse     = errors.New("role is in use")
	ErrRoleNotFound  = errors.New("role not found")
	ErrProtectedRole = errors.New("cannot modify protected role")

	// Token errors
	ErrTokenInvalid  = errors.New("token invalid")
	ErrTokenExpired  = errors.New("token expired")
	ErrTokenUsed     = errors.New("token already used")
	ErrTokenNotFound = errors.New("token not found")

	// Reset token errors
	ErrResetTokenInvalid = errors.New("invalid reset token")
	ErrResetTokenExpired = errors.New("reset token expired")
	ErrResetTokenUsed    = errors.New("reset token already used")

	// Zone errors
	ErrZoneNotFound = errors.New("zone not found")
	ErrZoneExists   = errors.New("zone already exists")
)
