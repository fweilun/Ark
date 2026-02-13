// README: Driver handlers for listing, accept, start.
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"

    "ark/internal/modules/matching"
    "ark/internal/modules/order"
    "ark/internal/types"
)

type DriverHandler struct {
	order    *order.Service
	matching *matching.Service
}

func NewDriverHandler(orderSvc *order.Service, matchingSvc *matching.Service) *DriverHandler {
	return &DriverHandler{order: orderSvc, matching: matchingSvc}
}

func (h *DriverHandler) ListAvailable(c *gin.Context) {
    // Matching not implemented; return empty list for MVP
    writeJSON(c, http.StatusOK, map[string]any{"orders": []any{}})
}

func (h *DriverHandler) Accept(c *gin.Context) {
    id := c.Param("id")
    if id == "" {
        writeError(c, http.StatusBadRequest, "missing order id")
        return
    }
    driverID := c.Query("driver_id")
    if driverID == "" {
        writeError(c, http.StatusBadRequest, "missing driver_id")
        return
    }
    err := h.order.Accept(c.Request.Context(), order.AcceptCommand{
        OrderID:  types.ID(id),
        DriverID: types.ID(driverID),
    })
    if err != nil {
        writeOrderError(c, err)
        return
    }
    writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusApproaching})
}

func (h *DriverHandler) Start(c *gin.Context) {
    id := c.Param("id")
    if id == "" {
        writeError(c, http.StatusBadRequest, "missing order id")
        return
    }
    err := h.order.Start(c.Request.Context(), order.StartCommand{OrderID: types.ID(id)})
    if err != nil {
        writeOrderError(c, err)
        return
    }
    writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusDriving})
}

func (h *DriverHandler) Complete(c *gin.Context) {
    id := c.Param("id")
    if id == "" {
        writeError(c, http.StatusBadRequest, "missing order id")
        return
    }
    err := h.order.Complete(c.Request.Context(), order.CompleteCommand{OrderID: types.ID(id)})
    if err != nil {
        writeOrderError(c, err)
        return
    }
    writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusPayment})
}

func (h *DriverHandler) Deny(c *gin.Context) {
    id := c.Param("id")
    if id == "" {
        writeError(c, http.StatusBadRequest, "missing order id")
        return
    }
    driverID := c.Query("driver_id")
    if driverID == "" {
        writeError(c, http.StatusBadRequest, "missing driver_id")
        return
    }
    err := h.order.Deny(c.Request.Context(), order.DenyCommand{
        OrderID:  types.ID(id),
        DriverID: types.ID(driverID),
    })
    if err != nil {
        writeOrderError(c, err)
        return
    }
    writeJSON(c, http.StatusOK, map[string]any{"status": order.StatusDenied})
}
