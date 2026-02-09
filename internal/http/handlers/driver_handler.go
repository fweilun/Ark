// README: Driver handlers for listing, accept, start.
package handlers

import (
    "net/http"

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

func (h *DriverHandler) ListAvailable(w http.ResponseWriter, r *http.Request) {
    // Matching not implemented; return empty list for MVP
    writeJSON(w, http.StatusOK, map[string]any{"orders": []any{}})
}

func (h *DriverHandler) Accept(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    if id == "" {
        writeError(w, http.StatusBadRequest, "missing order id")
        return
    }
    driverID := r.URL.Query().Get("driver_id")
    if driverID == "" {
        writeError(w, http.StatusBadRequest, "missing driver_id")
        return
    }
    err := h.order.Accept(r.Context(), order.AcceptCommand{
        OrderID:  types.ID(id),
        DriverID: types.ID(driverID),
    })
    if err != nil {
        writeOrderError(w, err)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"status": order.StatusAccepted})
}

func (h *DriverHandler) Start(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    if id == "" {
        writeError(w, http.StatusBadRequest, "missing order id")
        return
    }
    err := h.order.Start(r.Context(), order.StartCommand{OrderID: types.ID(id)})
    if err != nil {
        writeOrderError(w, err)
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{"status": order.StatusInProgress})
}
