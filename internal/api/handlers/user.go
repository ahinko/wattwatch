package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"wattwatch/internal/auth"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"

	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetUserFromContext retrieves the authenticated user from the gin context
func GetUserFromContext(c *gin.Context) *models.User {
	user, exists := c.Get("user")
	if !exists {
		return nil
	}
	if u, ok := user.(*models.User); ok {
		return u
	}
	return nil
}

type UserHandler struct {
	userRepo        repository.UserRepository
	authService     *auth.Service
	passwordHistory repository.PasswordHistoryRepository
	auditRepo       repository.AuditLogRepository
}

func NewUserHandler(userRepo repository.UserRepository, authService *auth.Service, passwordHistory repository.PasswordHistoryRepository, auditRepo repository.AuditLogRepository) *UserHandler {
	return &UserHandler{
		userRepo:        userRepo,
		authService:     authService,
		passwordHistory: passwordHistory,
		auditRepo:       auditRepo,
	}
}

// GetUser godoc
// @Summary Get user by ID
// @Description Get a user by their ID (requires auth, users can only access their own profile unless admin)
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID (UUID)"
// @Success 200 {object} models.User
// @Failure 400 {object} models.ErrorResponse "Invalid user ID"
// @Failure 403 {object} models.ErrorResponse "Permission denied - can only access own profile unless admin"
// @Failure 404 {object} models.ErrorResponse "User not found"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /users/{id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid user id"})
		return
	}

	// Get the authenticated user from context
	authUser := GetUserFromContext(c)
	if authUser == nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	// Get requested user
	requestedUser, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		switch err {
		case repository.ErrUserNotFound:
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
		default:
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
		}
		return
	}

	// Users can only access their own profile unless they're an admin
	if id != authUser.ID && !authUser.IsAdmin() {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "permission denied"})
		return
	}

	c.JSON(http.StatusOK, requestedUser)
}

// List godoc
// @Summary List users (Admin only)
// @Description List users with optional filtering. Requires admin privileges.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param search query string false "Search by username or email"
// @Param role_id query string false "Filter by role ID"
// @Param order_by query string false "Field to order by (username, email, created_at)"
// @Param order_desc query bool false "Order descending"
// @Param limit query int false "Limit results (default: 50)"
// @Param offset query int false "Offset results (default: 0)"
// @Success 200 {array} models.User
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Permission denied - admin only"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	// Get the authenticated user from context
	authUser := GetUserFromContext(c)
	if authUser == nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	// If not admin, only return their own user
	if !authUser.IsAdmin() {
		user, err := h.userRepo.GetByID(c.Request.Context(), authUser.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get user"})
			return
		}
		c.JSON(http.StatusOK, []models.User{*user})
		return
	}

	var filter repository.UserFilter

	// Parse query parameters
	if search := c.Query("search"); search != "" {
		filter.Search = &search
	}

	if roleID := c.Query("role_id"); roleID != "" {
		if id, err := uuid.Parse(roleID); err == nil {
			filter.RoleID = &id
		}
	}

	if orderBy := c.Query("order_by"); orderBy != "" {
		filter.OrderBy = orderBy
	}

	if orderDesc := c.Query("order_desc"); orderDesc == "true" {
		filter.OrderDesc = true
	}

	if limit := c.Query("limit"); limit != "" {
		if limitInt, err := strconv.Atoi(limit); err == nil {
			filter.Limit = &limitInt
		}
	}

	if offset := c.Query("offset"); offset != "" {
		if offsetInt, err := strconv.Atoi(offset); err == nil {
			filter.Offset = &offsetInt
		}
	}

	users, err := h.userRepo.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list users"})
		return
	}

	c.JSON(http.StatusOK, users)
}

// Update godoc
// @Summary Update user
// @Description Update a user's details
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID (UUID)"
// @Param request body models.UpdateUserRequest true "User details to update"
// @Success 200 {object} models.SuccessResponse "User updated successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid request"
// @Failure 404 {object} models.ErrorResponse "User not found"
// @Failure 409 {object} models.ErrorResponse "Email already exists"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /users/{id} [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	// Get the authenticated user from context
	authUser := GetUserFromContext(c)
	if authUser == nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid user id"})
		return
	}

	var req models.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Check if it's a JSON parsing error
		if _, ok := err.(*json.SyntaxError); ok {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
			return
		}
		// Check for invalid email
		if req.Email != nil && !auth.IsValidEmail(*req.Email) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid email address"})
			return
		}
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	// Get user to verify they exist
	user, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get user"})
		return
	}

	// Check permissions
	if id != authUser.ID && !authUser.IsAdmin() {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "permission denied"})
		return
	}

	// Non-admin users can't update roles or passwords
	if !authUser.IsAdmin() {
		if req.RoleID != nil {
			c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "only admins can change roles"})
			return
		}
		if req.Password != nil {
			c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "only admins can change passwords via this endpoint"})
			return
		}
	}

	// Update user fields
	if req.Email != nil {
		emailStr := *req.Email
		user.Email = &emailStr
	}
	if req.RoleID != nil {
		user.RoleID = *req.RoleID
	}
	if req.Password != nil {
		hashedPassword, err := h.authService.HashPassword(*req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to hash password"})
			return
		}
		user.Password = hashedPassword
	}

	// Update user
	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusConflict, models.ErrorResponse{Error: "email already exists"})
			return
		}
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update user"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// Delete godoc
// @Summary Delete user
// @Description Delete a user. Users can only delete their own account unless they are an admin.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID (UUID)"
// @Success 200 {object} models.SuccessResponse "User deleted successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid user ID"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Permission denied - can only delete own account unless admin"
// @Failure 404 {object} models.ErrorResponse "User not found"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Router /users/{id} [delete]
func (h *UserHandler) DeleteUser(c *gin.Context) {
	// Get the authenticated user from context
	authUser := GetUserFromContext(c)
	if authUser == nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid user id"})
		return
	}

	// Get user to verify they exist and check if they're an admin
	user, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get user"})
		return
	}

	// Check permissions
	if id != authUser.ID && !authUser.IsAdmin() {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "permission denied - can only delete own account unless admin"})
		return
	}

	// Check if trying to delete an admin user
	if user.Role != nil && user.Role.IsAdminGroup {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "cannot delete admin user"})
		return
	}

	// Create audit log before deleting the user
	if err := h.auditRepo.Create(c.Request.Context(), &models.CreateAuditLogRequest{
		UserID:      &authUser.ID,
		Action:      models.AuditActionDelete,
		EntityType:  "user",
		EntityID:    id.String(),
		Description: "User deleted",
		Metadata:    string(`{"user_id":"` + id.String() + `"}`),
		IPAddress:   c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
	}); err != nil {
		log.Printf("Error logging user deletion: %v", err)
	}

	// Delete user
	if err := h.userRepo.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "user deleted successfully"})
}

// ChangePassword godoc
// @Summary Change user password
// @Description Change a user's password (users can only change their own password)
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID (UUID)"
// @Param request body models.ChangePasswordRequest true "Password change details"
// @Success 200 {object} models.SuccessResponse "Password updated successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid request or password requirements not met"
// @Failure 403 {object} models.ErrorResponse "Permission denied - can only change own password"
// @Failure 404 {object} models.ErrorResponse "User not found"
// @Failure 409 {object} models.ErrorResponse "Password was recently used"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /users/{id}/password [put]
func (h *UserHandler) ChangePassword(c *gin.Context) {
	// Get the authenticated user from context
	authUser := GetUserFromContext(c)
	if authUser == nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid user id"})
		return
	}

	// Get user to verify they exist
	user, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get user"})
		return
	}

	// Only allow users to change their own password through this endpoint
	if id != authUser.ID {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "permission denied"})
		return
	}

	// Admin users should use the update endpoint to change passwords
	if authUser.IsAdmin() {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "admins must use the user update endpoint to change passwords"})
		return
	}

	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Verify current password
	if err := h.authService.ComparePasswords(user.Password, req.CurrentPassword); err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid current password"})
		return
	}

	// Hash new password
	hashedPassword, err := h.authService.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to hash password"})
		return
	}

	// Check password history
	if err := h.passwordHistory.CheckReuse(c.Request.Context(), id, req.NewPassword); err != nil {
		if errors.Is(err, repository.ErrPasswordReuse) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "password was recently used"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to check password history"})
		return
	}

	// Update password
	if err := h.userRepo.UpdatePassword(c.Request.Context(), id, hashedPassword); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update password"})
		return
	}

	// Add password to history
	if err := h.passwordHistory.Add(c.Request.Context(), id, hashedPassword); err != nil {
		log.Printf("Error adding password to history: %v", err)
	}

	// Log the password change
	if err := h.auditRepo.Create(c.Request.Context(), &models.CreateAuditLogRequest{
		UserID:      &authUser.ID,
		Action:      models.AuditActionUpdate,
		EntityType:  "user",
		EntityID:    id.String(),
		Description: "Password changed",
		Metadata:    string(`{"user_id":"` + id.String() + `"}`),
		IPAddress:   c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
	}); err != nil {
		log.Printf("Error logging password change: %v", err)
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "password changed successfully"})
}
