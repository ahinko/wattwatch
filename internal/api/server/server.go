// Package server provides the HTTP server implementation
package server

// @title           WattWatch API
// @version         1.0
// @description     WattWatch API server with global rate limiting.
// @x-skip-model-definitions true
//
// @description.markdown
// All API endpoints are subject to rate limiting:
// * Default rate: 100 requests per 60 seconds
// * Burst allowance: 5 additional requests
// * Rate limits are applied per IP address
//
// When rate limit is exceeded:
// * Status code 429 (Too Many Requests) is returned
// * Headers:
//   - X-RateLimit-Limit: Maximum requests allowed
//   - X-RateLimit-Reset: Unix timestamp when the rate limit resets
//   - Retry-After: Seconds to wait before retrying
//
// @host            localhost:8080
// @BasePath        /api/v1
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer token authentication
//
// @response 429 {object} models.ErrorResponse "Rate limit exceeded"

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"wattwatch/internal/api/routes"
	"wattwatch/internal/config"
)

// Server represents the HTTP server
type Server struct {
	cfg *config.Config
	db  *sql.DB
}

// New creates a new server instance
func New(cfg *config.Config, db *sql.DB) *Server {
	return &Server{
		cfg: cfg,
		db:  db,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Setup routes using the routes package
	router := routes.SetupRoutes(s.cfg, s.db)

	// Convert port string to int
	port, err := strconv.Atoi(s.cfg.API.Port)
	if err != nil {
		return fmt.Errorf("invalid port number: %w", err)
	}

	// Start server
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting server on %s", addr)
	return router.Run(addr)
}
