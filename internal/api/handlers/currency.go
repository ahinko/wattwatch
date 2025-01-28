package handlers

import (
	"net/http"
	"wattwatch/internal/models"
	"wattwatch/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CurrencyHandler handles currency-related requests
type CurrencyHandler struct {
	repo repository.CurrencyRepository
}

// NewCurrencyHandler creates a new CurrencyHandler
func NewCurrencyHandler(repo repository.CurrencyRepository) *CurrencyHandler {
	return &CurrencyHandler{repo: repo}
}

// ListCurrencies godoc
// @Summary List all currencies
// @Description Returns a list of all currencies
// @Tags currencies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.Currency
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal Server Error"
// @Router /currencies [get]
func (h *CurrencyHandler) ListCurrencies(c *gin.Context) {
	currencies, err := h.repo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch currencies"})
		return
	}

	c.JSON(http.StatusOK, currencies)
}

// GetCurrency godoc
// @Summary Get a currency by ID
// @Description Returns a currency by its ID
// @Tags currencies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Currency ID"
// @Success 200 {object} models.Currency
// @Failure 400 {object} models.ErrorResponse "Invalid currency ID"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 404 {object} models.ErrorResponse "Currency not found"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal Server Error"
// @Router /currencies/{id} [get]
func (h *CurrencyHandler) GetCurrency(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid currency ID"})
		return
	}

	currency, err := h.repo.GetByID(c.Request.Context(), id)
	if err == repository.ErrNotFound {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Currency not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to fetch currency"})
		return
	}

	c.JSON(http.StatusOK, currency)
}

// CreateCurrency godoc
// @Summary Create a new currency
// @Description Creates a new currency
// @Tags currencies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param currency body models.Currency true "Currency to create"
// @Success 201 {object} models.Currency
// @Failure 400 {object} models.ErrorResponse "Invalid request body"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal Server Error"
// @Router /currencies [post]
func (h *CurrencyHandler) CreateCurrency(c *gin.Context) {
	var currency models.Currency
	if err := c.ShouldBindJSON(&currency); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body"})
		return
	}

	if err := h.repo.Create(c.Request.Context(), &currency); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to create currency"})
		return
	}

	c.JSON(http.StatusCreated, currency)
}

// UpdateCurrency godoc
// @Summary Update a currency
// @Description Updates an existing currency
// @Tags currencies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Currency ID"
// @Param currency body models.Currency true "Updated currency"
// @Success 200 {object} models.Currency
// @Failure 400 {object} models.ErrorResponse "Invalid request body or currency ID"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 404 {object} models.ErrorResponse "Currency not found"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal Server Error"
// @Router /currencies/{id} [put]
func (h *CurrencyHandler) UpdateCurrency(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid currency ID"})
		return
	}

	var currency models.Currency
	if err := c.ShouldBindJSON(&currency); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid request body"})
		return
	}

	currency.ID = id
	if err := h.repo.Update(c.Request.Context(), &currency); err == repository.ErrNotFound {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Currency not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to update currency"})
		return
	}

	c.JSON(http.StatusOK, currency)
}

// DeleteCurrency godoc
// @Summary Delete a currency
// @Description Deletes an existing currency
// @Tags currencies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Currency ID"
// @Success 204 "No Content"
// @Failure 400 {object} models.ErrorResponse "Invalid currency ID"
// @Failure 401 {object} models.ErrorResponse "Unauthorized"
// @Failure 404 {object} models.ErrorResponse "Currency not found"
// @Failure 429 {object} models.ErrorResponse "Rate limit exceeded"
// @Failure 500 {object} models.ErrorResponse "Internal Server Error"
// @Router /currencies/{id} [delete]
func (h *CurrencyHandler) DeleteCurrency(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "Invalid currency ID"})
		return
	}

	if err := h.repo.Delete(c.Request.Context(), id); err == repository.ErrNotFound {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "Currency not found"})
		return
	} else if err == repository.ErrHasAssociatedRecords {
		c.JSON(http.StatusConflict, models.ErrorResponse{Error: "cannot delete currency that has associated spot prices"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "Failed to delete currency"})
		return
	}

	c.Status(http.StatusNoContent)
}
