package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"
	"wattwatch/internal/config"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidToken indicates the token is invalid
	ErrInvalidToken = errors.New("invalid token")
	// ErrTokenExpired indicates the token has expired
	ErrTokenExpired = errors.New("token expired")
)

// Service provides authentication functionality
type Service struct {
	config           *config.Config
	refreshTokenRepo repository.RefreshTokenRepository
}

// NewService creates a new authentication service
func NewService(config *config.Config, refreshTokenRepo repository.RefreshTokenRepository) *Service {
	return &Service{
		config:           config,
		refreshTokenRepo: refreshTokenRepo,
	}
}

// GenerateToken generates a new JWT token
func (s *Service) GenerateToken(user *models.User, isRefresh bool) (string, error) {
	// Set expiration based on token type
	var expiration time.Duration
	if isRefresh {
		expiration = time.Hour * 24 * 7 // 7 days for refresh token
	} else {
		expiration = time.Minute * 15 // 15 minutes for access token
	}

	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"is_admin": user.Role.IsAdminGroup,
		"exp":      time.Now().Add(expiration).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

// GenerateRefreshToken generates a new refresh token
func (s *Service) GenerateRefreshToken(ctx context.Context, userID uuid.UUID) (string, error) {
	// Generate random bytes for the token
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	// Convert to base64
	token := base64.URLEncoding.EncodeToString(b)

	// Set expiration to 7 days
	expiresAt := time.Now().Add(time.Hour * 24 * 7)

	// Store in database
	if err := s.refreshTokenRepo.Create(ctx, userID, token, expiresAt); err != nil {
		return "", err
	}

	return token, nil
}

// ValidateRefreshToken validates a refresh token and returns the associated user ID
func (s *Service) ValidateRefreshToken(token string) (uuid.UUID, error) {
	refreshToken, err := s.refreshTokenRepo.GetByToken(context.Background(), token)
	if err != nil {
		if errors.Is(err, repository.ErrTokenNotFound) {
			return uuid.Nil, ErrInvalidToken
		}
		if errors.Is(err, repository.ErrTokenExpired) {
			return uuid.Nil, ErrTokenExpired
		}
		return uuid.Nil, err
	}

	return refreshToken.UserID, nil
}

// DeleteRefreshToken removes a refresh token
func (s *Service) DeleteRefreshToken(token string) error {
	return s.refreshTokenRepo.DeleteByToken(context.Background(), token)
}

// DeleteAllRefreshTokens removes all refresh tokens for a user
func (s *Service) DeleteAllRefreshTokens(userID uuid.UUID) error {
	return s.refreshTokenRepo.DeleteByUserID(context.Background(), userID)
}

// HashPassword hashes a password using bcrypt
func (s *Service) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// ComparePasswords compares a hashed password with a plain text password
func (s *Service) ComparePasswords(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// ValidateToken validates a JWT token and returns the claims
func (s *Service) ValidateToken(tokenString string) (*jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		if err == jwt.ErrTokenExpired {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return &claims, nil
	}

	return nil, ErrInvalidToken
}

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
