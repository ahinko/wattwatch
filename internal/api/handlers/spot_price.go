package handlers

import (
	"net/http"
	"time"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SpotPriceHandler handles spot price-related requests
type SpotPriceHandler struct {
	repo         repository.SpotPriceRepository
	zoneRepo     repository.ZoneRepository
	currencyRepo repository.CurrencyRepository
}

// NewSpotPriceHandler creates a new SpotPriceHandler
func NewSpotPriceHandler(repo repository.SpotPriceRepository, zoneRepo repository.ZoneRepository, currencyRepo repository.CurrencyRepository) *SpotPriceHandler {
	return &SpotPriceHandler{
		repo:         repo,
		zoneRepo:     zoneRepo,
		currencyRepo: currencyRepo,
	}
}

// ListSpotPrices godoc
// @Summary List spot prices
// @Description Returns a list of spot prices for a specific zone and currency within a date range (max 7 days)
// @Tags spot-prices
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param zone query string true "Zone name (e.g., 'SE1')"
// @Param currency query string true "Currency name (e.g., 'EUR')"
// @Param start_time query string true "Start time (RFC3339)"
// @Param end_time query string true "End time (RFC3339)"
// @Param order_desc query boolean false "Order descending"
// @Success 200 {array} models.SpotPrice
// @Failure 400 {object} models.ErrorResponse "Invalid parameters or date range exceeds 7 days"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 404 {object} models.ErrorResponse "Zone or currency not found"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal Server Error"
// @Router /spot-prices [get]
func (h *SpotPriceHandler) ListSpotPrices(c *gin.Context) {
	filter := repository.SpotPriceFilter{}

	// Parse zone name and get ID
	zoneName := c.Query("zone")
	if zoneName == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "zone is required"})
		return
	}
	zone, err := h.zoneRepo.GetByName(c.Request.Context(), zoneName)
	if err == repository.ErrNotFound {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "zone not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch zone"})
		return
	}
	filter.ZoneID = &zone.ID

	// Parse currency name and get ID
	currencyName := c.Query("currency")
	if currencyName == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "currency is required"})
		return
	}
	currency, err := h.currencyRepo.GetByName(c.Request.Context(), currencyName)
	if err == repository.ErrNotFound {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "currency not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch currency"})
		return
	}
	filter.CurrencyID = &currency.ID

	// Parse start_time (required)
	startTimeStr := c.Query("start_time")
	if startTimeStr == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "start_time is required"})
		return
	}
	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid start time format, use RFC3339"})
		return
	}
	filter.StartTime = &startTime

	// Parse end_time (required)
	endTimeStr := c.Query("end_time")
	if endTimeStr == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "end_time is required"})
		return
	}
	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid end time format, use RFC3339"})
		return
	}
	filter.EndTime = &endTime

	// Validate date range (max 7 days)
	if endTime.Sub(startTime) > 7*24*time.Hour {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "date range cannot exceed 7 days"})
		return
	}

	if endTime.Before(startTime) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "end_time must be after start_time"})
		return
	}

	// Set default ordering to timestamp ascending if not specified
	filter.OrderBy = "timestamp"
	if desc := c.Query("order_desc"); desc == "true" {
		filter.OrderDesc = true
	}

	// Set maximum limit
	maxLimit := 1000 // This allows for up to ~6 prices per hour for 7 days
	filter.Limit = &maxLimit

	spotPrices, err := h.repo.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to fetch spot prices"})
		return
	}

	c.JSON(http.StatusOK, spotPrices)
}

// GetSpotPrice godoc
// @Summary Get a spot price by ID
// @Description Returns a spot price by its ID
// @Tags spot-prices
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Spot Price ID"
// @Success 200 {object} models.SpotPrice
// @Failure 400 {object} models.ErrorResponse "Invalid spot price ID"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 404 {object} models.ErrorResponse "Spot price not found"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal Server Error"
// @Router /spot-prices/{id} [get]
func (h *SpotPriceHandler) GetSpotPrice(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid spot price ID"})
		return
	}

	spotPrice, err := h.repo.GetByID(c.Request.Context(), id)
	if err == repository.ErrNotFound {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Spot price not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch spot price"})
		return
	}

	c.JSON(http.StatusOK, spotPrice)
}

// CreateSpotPrices godoc
// @Summary Create or update spot prices (Admin only)
// @Description Creates or updates one or more spot prices in a single transaction. If a spot price with the same timestamp, zone_id, and currency_id exists, its price will be updated. Requires admin privileges.
// @Tags spot-prices
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param spot_prices body models.CreateSpotPricesRequest true "Spot prices to create or update"
// @Success 201 {array} models.SpotPrice
// @Failure 400 {object} models.ErrorResponse "Invalid request body, negative price, or invalid zone/currency"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Permission denied - admin only"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal Server Error"
// @Router /spot-prices [post]
func (h *SpotPriceHandler) CreateSpotPrices(c *gin.Context) {
	var req models.CreateSpotPricesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body"})
		return
	}

	if len(req.SpotPrices) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "at least one spot price is required"})
		return
	}

	// Convert request to spot prices
	spotPrices := make([]models.SpotPrice, len(req.SpotPrices))
	for i, sp := range req.SpotPrices {
		if sp.Price < 0 {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "price cannot be negative"})
			return
		}

		// Validate zone ID exists
		if _, err := h.zoneRepo.GetByID(c.Request.Context(), sp.ZoneID); err == repository.ErrNotFound {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid zone id"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to validate zone"})
			return
		}

		// Validate currency ID exists
		if _, err := h.currencyRepo.GetByID(c.Request.Context(), sp.CurrencyID); err == repository.ErrNotFound {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid currency id"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to validate currency"})
			return
		}

		spotPrices[i] = models.SpotPrice{
			ID:         uuid.New(),
			Timestamp:  sp.Timestamp,
			ZoneID:     sp.ZoneID,
			CurrencyID: sp.CurrencyID,
			Price:      sp.Price,
		}
	}

	if err := h.repo.CreateBatch(c.Request.Context(), spotPrices); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create spot prices"})
		return
	}

	c.JSON(http.StatusCreated, spotPrices)
}

// DeleteSpotPrice godoc
// @Summary Delete a spot price (Admin only)
// @Description Deletes an existing spot price. Requires admin privileges.
// @Tags spot-prices
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Spot Price ID"
// @Success 200 {object} models.SuccessResponse "Spot price deleted successfully"
// @Failure 400 {object} models.ErrorResponse "Invalid spot price ID"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Permission denied - admin only"
// @Failure 404 {object} models.ErrorResponse "Spot price not found"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal Server Error"
// @Router /spot-prices/{id} [delete]
func (h *SpotPriceHandler) DeleteSpotPrice(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid spot price ID"})
		return
	}

	if err := h.repo.Delete(c.Request.Context(), id); err == repository.ErrNotFound {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Spot price not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to delete spot price"})
		return
	}

	c.Status(http.StatusNoContent)
}
