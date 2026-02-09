// README: Location handlers (MVP).
package handlers

import (
    "net/http"

    "ark/internal/modules/location"
    "ark/internal/types"
)

type LocationHandler struct {
    location *location.Service
}

func NewLocationHandler(svc *location.Service) *LocationHandler {
    return &LocationHandler{location: svc}
}

func (h *LocationHandler) Update(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    if id == "" {
        writeError(w, http.StatusBadRequest, "missing id")
        return
    }
    // For MVP, no body parsing yet
    _ = h.location.Update(r.Context(), location.Update{UserID: types.ID(id), UserType: "driver"})
    writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
