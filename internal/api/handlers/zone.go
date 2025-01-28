package handlers

import (
	"net/http"
	"strconv"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ZoneHandler handles zone-related requests
type ZoneHandler struct {
	repo repository.ZoneRepository
}

// NewZoneHandler creates a new ZoneHandler
func NewZoneHandler(repo repository.ZoneRepository) *ZoneHandler {
	return &ZoneHandler{repo: repo}
}

// ListZones godoc
// @Summary List all zones
// @Description Returns a list of all zones
// @Tags zones
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param search query string false "Search zones by name"
// @Param order_by query string false "Order by field (name, timezone)"
// @Param order_desc query boolean false "Order descending"
// @Param limit query integer false "Limit results"
// @Param offset query integer false "Offset results"
// @Success 200 {array} models.Zone
// @Failure 400 {object} models.ErrorResponse "Invalid parameters"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal Server Error"
// @Router /zones [get]
func (h *ZoneHandler) ListZones(c *gin.Context) {
	filter := repository.ZoneFilter{}

	// Parse search
	if search := c.Query("search"); search != "" {
		filter.Search = &search
	}

	// Parse ordering
	if orderBy := c.Query("order_by"); orderBy != "" {
		filter.OrderBy = orderBy
		if desc := c.Query("order_desc"); desc != "" {
			filter.OrderDesc = desc == "true"
		}
	}

	// Parse pagination
	if limitStr := c.Query("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 0 {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid limit"})
			return
		}
		filter.Limit = &limit
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid offset"})
			return
		}
		filter.Offset = &offset
	}

	zones, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch zones"})
		return
	}

	c.JSON(http.StatusOK, zones)
}

// GetZone godoc
// @Summary Get a zone by ID
// @Description Returns a zone by its ID
// @Tags zones
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Zone ID"
// @Success 200 {object} models.Zone
// @Failure 400 {object} models.ErrorResponse "Invalid zone ID"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 404 {object} models.ErrorResponse "Zone not found"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal Server Error"
// @Router /zones/{id} [get]
func (h *ZoneHandler) GetZone(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid zone ID"})
		return
	}

	zone, err := h.repo.GetByID(c.Request.Context(), id)
	if err == repository.ErrNotFound {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Zone not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch zone"})
		return
	}

	c.JSON(http.StatusOK, zone)
}

// CreateZone godoc
// @Summary Create a new zone
// @Description Creates a new zone
// @Tags zones
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param zone body models.Zone true "Zone to create"
// @Success 201 {object} models.Zone
// @Failure 400 {object} models.ErrorResponse "Invalid request body"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal Server Error"
// @Router /zones [post]
func (h *ZoneHandler) CreateZone(c *gin.Context) {
	var zone models.Zone
	if err := c.ShouldBindJSON(&zone); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body"})
		return
	}

	if err := h.repo.Create(c.Request.Context(), &zone); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create zone"})
		return
	}

	c.JSON(http.StatusCreated, zone)
}

// UpdateZone godoc
// @Summary Update a zone
// @Description Updates an existing zone
// @Tags zones
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Zone ID"
// @Param zone body models.Zone true "Updated zone"
// @Success 200 {object} models.Zone
// @Failure 400 {object} models.ErrorResponse "Invalid request body or zone ID"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 404 {object} models.ErrorResponse "Zone not found"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal Server Error"
// @Router /zones/{id} [put]
func (h *ZoneHandler) UpdateZone(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid zone ID"})
		return
	}

	var zone models.Zone
	if err := c.ShouldBindJSON(&zone); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body"})
		return
	}

	zone.ID = id
	if err := h.repo.Update(c.Request.Context(), &zone); err == repository.ErrNotFound {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Zone not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update zone"})
		return
	}

	c.JSON(http.StatusOK, zone)
}

// DeleteZone godoc
// @Summary Delete a zone
// @Description Deletes an existing zone
// @Tags zones
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Zone ID"
// @Success 204 "No Content"
// @Failure 400 {object} models.ErrorResponse "Invalid zone ID"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 404 {object} models.ErrorResponse "Zone not found"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal Server Error"
// @Router /zones/{id} [delete]
func (h *ZoneHandler) DeleteZone(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid zone ID"})
		return
	}

	if err := h.repo.Delete(c.Request.Context(), id); err == repository.ErrNotFound {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Zone not found"})
		return
	} else if err == repository.ErrHasAssociatedRecords {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "cannot delete zone that has associated spot prices"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to delete zone"})
		return
	}

	c.Status(http.StatusNoContent)
}
