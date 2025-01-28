package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
	"wattwatch/internal/models"
	"wattwatch/internal/provider"

	"github.com/gin-gonic/gin"
)

// ProviderHandler handles provider-related requests
type ProviderHandler struct {
	manager *provider.Manager
}

// NewProviderHandler creates a new ProviderHandler
func NewProviderHandler(manager *provider.Manager) *ProviderHandler {
	return &ProviderHandler{
		manager: manager,
	}
}

// TriggerNordpoolFetchRequest represents the request body for triggering nordpool fetch
type TriggerNordpoolFetchRequest struct {
	StartDate  time.Time `json:"start_date" binding:"required"`
	EndDate    time.Time `json:"end_date" binding:"required"`
	Zones      []string  `json:"zones"`
	Currencies []string  `json:"currencies"`
}

// TriggerNordpoolFetchResponse represents the response for triggering nordpool fetch
type TriggerNordpoolFetchResponse struct {
	Message string `json:"message"`
}

// TriggerNordpoolFetch godoc
// @Summary Trigger nordpool provider fetch (Admin only)
// @Description Triggers the nordpool provider to fetch spot prices for specified dates, zones, and currencies. Maximum date range is 14 days.
// @Tags providers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body TriggerNordpoolFetchRequest true "Fetch request parameters"
// @Success 202 {object} TriggerNordpoolFetchResponse
// @Failure 400 {object} models.ErrorResponse "Invalid request body or parameters"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 403 {object} models.ErrorResponse "Permission denied - admin only"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal Server Error"
// @Router /providers/nordpool/fetch [post]
func (h *ProviderHandler) TriggerNordpoolFetch(c *gin.Context) {
	var req TriggerNordpoolFetchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body"})
		return
	}

	// Validate date range
	if req.EndDate.Before(req.StartDate) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "end_date must be after start_date"})
		return
	}

	if req.EndDate.Sub(req.StartDate) > 14*24*time.Hour {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "date range cannot exceed 14 days"})
		return
	}

	// Get nordpool provider
	nordpoolProvider, exists := h.manager.GetProvider("nordpool")
	if !exists {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "nordpool provider not found"})
		return
	}

	// Get supported zones and currencies if not specified
	config := nordpoolProvider.GetConfig()
	if len(req.Zones) == 0 {
		req.Zones = config.SupportedZones
	}
	if len(req.Currencies) == 0 {
		req.Currencies = config.SupportedCurrencies
	}

	// Validate zones and currencies
	for _, zone := range req.Zones {
		if !nordpoolProvider.SupportsZone(zone) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: fmt.Sprintf("unsupported zone: %s", zone)})
			return
		}
	}
	for _, currency := range req.Currencies {
		if !nordpoolProvider.SupportsCurrency(currency) {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: fmt.Sprintf("unsupported currency: %s", currency)})
			return
		}
	}

	// Start background job
	go func() {
		currentDate := req.StartDate
		for currentDate.Before(req.EndDate) || currentDate.Equal(req.EndDate) {
			for _, zone := range req.Zones {
				for _, currency := range req.Currencies {
					opts := provider.RunOptions{
						Date:     currentDate,
						Zone:     zone,
						Currency: currency,
					}

					if err := h.manager.RunProvider(context.Background(), "nordpool", &opts); err != nil {
						log.Printf("Error running nordpool provider for date %s, zone %s, currency %s: %v",
							currentDate.Format("2006-01-02"), zone, currency, err)
					}

					// Sleep for 5 seconds between requests
					time.Sleep(5 * time.Second)
				}
			}
			currentDate = currentDate.Add(24 * time.Hour)
		}
	}()

	c.JSON(http.StatusAccepted, TriggerNordpoolFetchResponse{
		Message: "Nordpool fetch request queued successfully",
	})
}
