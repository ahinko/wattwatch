package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"wattwatch/internal/auth"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RoleHandler struct {
	roleRepo  repository.RoleRepository
	userRepo  repository.UserRepository
	auditRepo repository.AuditLogRepository
}

func NewRoleHandler(roleRepo repository.RoleRepository, userRepo repository.UserRepository, auditRepo repository.AuditLogRepository) *RoleHandler {
	return &RoleHandler{
		roleRepo:  roleRepo,
		userRepo:  userRepo,
		auditRepo: auditRepo,
	}
}

// GetRole godoc
// @Summary Get role by ID
// @Description Get a role by its ID (admin only)
// @Tags roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID (UUID)"
// @Success 200 {object} models.Role
// @Failure 400 {object} models.ErrorResponse "Invalid role ID"
// @Failure 403 {object} models.ErrorResponse "Permission denied - admin only"
// @Failure 404 {object} models.ErrorResponse "Role not found"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /roles/{id} [get]
func (h *RoleHandler) GetRole(c *gin.Context) {
	// Get the authenticated user from context
	authUser := auth.GetUserFromContext(c)
	if authUser == nil || !authUser.IsAdmin() {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "permission denied"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid role ID"})
		return
	}

	role, err := h.roleRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		log.Printf("Error getting role: %v", err)
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "role not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get role"})
		return
	}

	c.JSON(http.StatusOK, role)
}

// List godoc
// @Summary List roles
// @Description List all roles with optional filtering
// @Tags roles
// @Accept json
// @Produce json
// @Param search query string false "Search term for role name"
// @Param protected query bool false "Filter by protected status"
// @Param admin_group query bool false "Filter by admin group status"
// @Param order_by query string false "Field to order by"
// @Param order_desc query bool false "Order descending"
// @Param limit query int false "Limit number of results"
// @Param offset query int false "Offset results"
// @Success 200 {array} models.Role
// @Failure 400 {object} models.ErrorResponse "Invalid request"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /roles [get]
func (h *RoleHandler) ListRoles(c *gin.Context) {
	// Get the authenticated user from context
	authUser := auth.GetUserFromContext(c)
	if authUser == nil {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "permission denied"})
		return
	}

	// If not admin, only return their own role
	if !authUser.IsAdmin() {
		role, err := h.roleRepo.GetByID(c.Request.Context(), authUser.RoleID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
			return
		}
		c.JSON(http.StatusOK, []models.Role{*role})
		return
	}

	var filter repository.RoleFilter

	// Parse query parameters
	if search := c.Query("search"); search != "" {
		filter.Search = &search
	}
	if protected := c.Query("protected"); protected != "" {
		protectedBool, err := strconv.ParseBool(protected)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid protected parameter"})
			return
		}
		filter.Protected = &protectedBool
	}
	if adminGroup := c.Query("admin_group"); adminGroup != "" {
		adminGroupBool, err := strconv.ParseBool(adminGroup)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid admin_group parameter"})
			return
		}
		filter.AdminGroup = &adminGroupBool
	}
	filter.OrderBy = c.Query("order_by")
	if orderDesc := c.Query("order_desc"); orderDesc != "" {
		orderDescBool, err := strconv.ParseBool(orderDesc)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid order_desc parameter"})
			return
		}
		filter.OrderDesc = orderDescBool
	}
	if limit := c.Query("limit"); limit != "" {
		limitInt, err := strconv.Atoi(limit)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid limit parameter"})
			return
		}
		filter.Limit = &limitInt
	}
	if offset := c.Query("offset"); offset != "" {
		offsetInt, err := strconv.Atoi(offset)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid offset parameter"})
			return
		}
		filter.Offset = &offsetInt
	}

	roles, err := h.roleRepo.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list roles"})
		return
	}

	c.JSON(http.StatusOK, roles)
}

// CreateRole godoc
// @Summary Create role
// @Description Create a new role (admin only)
// @Tags roles
// @Accept json
// @Produce json
// @Param role body models.CreateRoleRequest true "Role details"
// @Success 201 {object} models.Role
// @Failure 400 {object} models.ErrorResponse "Invalid request body"
// @Failure 403 {object} models.ErrorResponse "Permission denied - admin only"
// @Failure 409 {object} models.ErrorResponse "Role already exists"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /roles [post]
func (h *RoleHandler) CreateRole(c *gin.Context) {
	// Get the authenticated user from context
	authUser := auth.GetUserFromContext(c)
	if authUser == nil || !authUser.IsAdmin() {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "permission denied"})
		return
	}

	var req models.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	role := &models.Role{
		Name:         req.Name,
		IsProtected:  req.IsProtected,
		IsAdminGroup: req.IsAdminGroup,
	}

	if err := h.roleRepo.Create(c.Request.Context(), role); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "role name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create role"})
		return
	}

	// Log the creation
	if err := h.auditRepo.Create(c.Request.Context(), &models.CreateAuditLogRequest{
		UserID:      &authUser.ID,
		Action:      models.AuditActionCreate,
		EntityType:  "role",
		EntityID:    role.ID.String(),
		Description: "Role created",
		Metadata:    string(`{"role_id":"` + role.ID.String() + `"}`),
		IPAddress:   c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
	}); err != nil {
		log.Printf("Error logging role creation: %v", err)
	}

	c.JSON(http.StatusCreated, role)
}

// UpdateRole godoc
// @Summary Update role
// @Description Update a role's details (admin only)
// @Tags roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID (UUID)"
// @Param role body models.UpdateRoleRequest true "Role details"
// @Success 200 {object} models.Role
// @Failure 400 {object} models.ErrorResponse "Invalid request body or role ID"
// @Failure 403 {object} models.ErrorResponse "Permission denied - admin only"
// @Failure 404 {object} models.ErrorResponse "Role not found"
// @Failure 409 {object} models.ErrorResponse "Role already exists"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /roles/{id} [put]
func (h *RoleHandler) UpdateRole(c *gin.Context) {
	// Get the authenticated user from context
	authUser := auth.GetUserFromContext(c)
	if authUser == nil || !authUser.IsAdmin() {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "permission denied"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid role ID"})
		return
	}
	if id == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid role ID"})
		return
	}

	var req models.UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Get existing role
	role, err := h.roleRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		log.Printf("Error getting role: %v", err)
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "role not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to get role"})
		return
	}

	// Update fields
	role.Name = req.Name
	role.IsProtected = req.IsProtected
	role.IsAdminGroup = req.IsAdminGroup

	if err := h.roleRepo.Update(c.Request.Context(), role); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "role name already exists"})
			return
		}
		if errors.Is(err, repository.ErrProtectedRole) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "cannot modify protected role"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update role"})
		return
	}

	// Log the update
	if err := h.auditRepo.Create(c.Request.Context(), &models.CreateAuditLogRequest{
		UserID:      &authUser.ID,
		Action:      models.AuditActionUpdate,
		EntityType:  "role",
		EntityID:    role.ID.String(),
		Description: "Role updated",
		Metadata:    string(`{"role_id":"` + role.ID.String() + `"}`),
		IPAddress:   c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
	}); err != nil {
		log.Printf("Error logging role update: %v", err)
	}

	c.JSON(http.StatusOK, role)
}

// DeleteRole godoc
// @Summary Delete role
// @Description Delete a role (admin only)
// @Tags roles
// @Accept json
// @Produce json
// @Param id path string true "Role ID (UUID)"
// @Success 200 {object} models.SuccessResponse "Role deleted successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid role ID"
// @Failure 403 {object} models.ErrorResponse "Permission denied - admin only"
// @Failure 404 {object} models.ErrorResponse "Role not found"
// @Failure 409 {object} models.ErrorResponse "Role in use by users"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal server error"
// @Security BearerAuth
// @Router /roles/{id} [delete]
func (h *RoleHandler) DeleteRole(c *gin.Context) {
	// Get the authenticated user from context
	authUser := auth.GetUserFromContext(c)
	if authUser == nil || !authUser.IsAdmin() {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "permission denied"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil || id == uuid.Nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid role id"})
		return
	}

	// Delete the role
	if err := h.roleRepo.Delete(c.Request.Context(), id); err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "role not found"})
		case errors.Is(err, repository.ErrRoleInUse), errors.Is(err, repository.ErrHasAssociatedRecords):
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "cannot delete role with assigned users"})
		case errors.Is(err, repository.ErrProtectedRole):
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "cannot delete protected role"})
		default:
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
		}
		return
	}

	// Log the deletion
	if err := h.auditRepo.Create(c.Request.Context(), &models.CreateAuditLogRequest{
		UserID:      &authUser.ID,
		Action:      models.AuditActionDelete,
		EntityType:  "role",
		EntityID:    id.String(),
		Description: "Role deleted",
		Metadata:    string(`{"role_id":"` + id.String() + `"}`),
		IPAddress:   c.ClientIP(),
		UserAgent:   c.GetHeader("User-Agent"),
	}); err != nil {
		log.Printf("Error logging role deletion: %v", err)
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "role deleted successfully"})
}
