package middleware

import (
	"net/http"
	"strings"
	"wattwatch/internal/auth"
	"wattwatch/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthMiddleware struct {
	authService *auth.Service
	userRepo    repository.UserRepository
	roleRepo    repository.RoleRepository
}

func NewAuthMiddleware(authService *auth.Service, userRepo repository.UserRepository, roleRepo repository.RoleRepository) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
		userRepo:    userRepo,
		roleRepo:    roleRepo,
	}
}

func (m *AuthMiddleware) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no authorization header"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			c.Abort()
			return
		}

		claims, err := m.authService.ValidateToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		// Get user ID from claims
		userIDStr, ok := (*claims)["user_id"].(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			c.Abort()
			return
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id in token"})
			c.Abort()
			return
		}

		// Get full user object from database
		user, err := m.userRepo.GetByID(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
			c.Abort()
			return
		}

		// Get user's role
		role, err := m.roleRepo.GetByID(c.Request.Context(), user.RoleID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user role"})
			c.Abort()
			return
		}
		user.Role = role

		// Store full user object in context
		c.Set("user", user)
		c.Set("is_admin", user.Role.IsAdminGroup)

		c.Next()
	}
}

func (m *AuthMiddleware) AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get("is_admin")
		if !exists || !isAdmin.(bool) {
			c.JSON(http.StatusForbidden, gin.H{"error": "admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}
