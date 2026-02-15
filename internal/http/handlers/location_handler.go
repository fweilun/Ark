// README: Location handlers (Gin implementation).
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"ark/internal/modules/location"
)

type LocationHandler struct {
	location *location.Service
}

func NewLocationHandler(svc *location.Service) *LocationHandler {
	return &LocationHandler{location: svc}
}

func (h *LocationHandler) UpdateDriver(c *gin.Context) {
	var req location.DriverLocationUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	result, err := h.location.UpdateDriverLocation(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, location.ErrValidation) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *LocationHandler) UpdatePassenger(c *gin.Context) {
	var req location.LocationUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	result, err := h.location.UpdatePassengerLocation(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, location.ErrValidation) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, result)
}
