// README: HTTP helper utilities for JSON and error mapping.
package http

import (
    "encoding/json"
    "net/http"

    "ark/internal/modules/order"
)

type errorResponse struct {
    Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
    writeJSON(w, status, errorResponse{Error: msg})
}

func writeOrderError(w http.ResponseWriter, err error) {
    switch err {
    case order.ErrBadRequest:
        writeError(w, http.StatusBadRequest, err.Error())
    case order.ErrNotFound:
        writeError(w, http.StatusNotFound, err.Error())
    case order.ErrInvalidState, order.ErrActiveOrder:
        writeError(w, http.StatusConflict, err.Error())
    case order.ErrConflict:
        writeError(w, http.StatusConflict, err.Error())
    default:
        writeError(w, http.StatusInternalServerError, "internal error")
    }
}
