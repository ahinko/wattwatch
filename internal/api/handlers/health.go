package handlers

import (
	"database/sql"
	"net/http"
	"time"
	"wattwatch/internal/models"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	db *sql.DB
}

func NewHealthHandler(db *sql.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

// Health godoc
// @Summary Health check
// @Description Returns the health status of the API and its dependencies
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} models.HealthResponse
// @Failure 503 {object} models.ErrorResponse "Service unavailable"
// @Router /health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	// Check database connection
	if err := h.db.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "database connection failed"})
		return
	}

	c.JSON(http.StatusOK, models.HealthResponse{
		Status: "healthy",
		Time:   time.Now().UTC(),
	})
}
