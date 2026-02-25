// README: Payment HTTP handlers — client-facing endpoints for payment operations.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ark/internal/modules/payment"
)

// PaymentHandler handles HTTP requests for the payment module.
type PaymentHandler struct {
	svc payment.Service
}

// NewPaymentHandler creates a PaymentHandler backed by the given Service.
func NewPaymentHandler(svc payment.Service) *PaymentHandler {
	return &PaymentHandler{svc: svc}
}

type createPaymentReq struct {
	TripID        int64   `json:"trip_id"`
	PaymentMethod string  `json:"payment_method"`
	Amount        float64 `json:"amount"`
}

// Create handles POST /api/v1/payments.
func (h *PaymentHandler) Create(c *gin.Context) {
	var req createPaymentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.TripID <= 0 || req.Amount <= 0 || req.PaymentMethod == "" {
		writeError(c, http.StatusBadRequest, "missing or invalid fields")
		return
	}

	method := payment.PaymentMethod(req.PaymentMethod)
	if method != payment.CreditCard && method != payment.Wallet {
		writeError(c, http.StatusBadRequest, "invalid payment_method: must be credit_card or wallet")
		return
	}

	p, err := h.svc.CreatePayment(c.Request.Context(), req.TripID, method, req.Amount)
	if err != nil {
		writePaymentError(c, err)
		return
	}
	writeJSON(c, http.StatusCreated, paymentResponse(p))
}

// GetByID handles GET /api/v1/payments/:paymentId.
func (h *PaymentHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("paymentId"), 10, 64)
	if err != nil || id <= 0 {
		writeError(c, http.StatusBadRequest, "invalid payment id")
		return
	}

	p, err := h.svc.GetPayment(c.Request.Context(), id)
	if err != nil {
		writePaymentError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, paymentResponse(p))
}

// GetByTrip handles GET /api/v1/trips/:tripId/payments.
func (h *PaymentHandler) GetByTrip(c *gin.Context) {
	tripID, err := strconv.ParseInt(c.Param("tripId"), 10, 64)
	if err != nil || tripID <= 0 {
		writeError(c, http.StatusBadRequest, "invalid trip id")
		return
	}

	payments, err := h.svc.GetPaymentsByTrip(c.Request.Context(), tripID)
	if err != nil {
		writePaymentError(c, err)
		return
	}

	resp := make([]map[string]any, 0, len(payments))
	for _, p := range payments {
		resp = append(resp, paymentResponse(p))
	}
	writeJSON(c, http.StatusOK, resp)
}

// Callback handles POST /api/v1/payments/callback.
func (h *PaymentHandler) Callback(c *gin.Context) {
	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.svc.HandleCallback(c.Request.Context(), data); err != nil {
		writePaymentError(c, err)
		return
	}
	writeJSON(c, http.StatusOK, map[string]any{"ok": true})
}

func paymentResponse(p *payment.Payment) map[string]any {
	resp := map[string]any{
		"payment_id":     p.PaymentID,
		"trip_id":        p.TripID,
		"payment_method": p.PaymentMethod,
		"amount":         p.Amount,
		"status":         p.Status,
	}
	if p.PaidAt != nil {
		resp["paid_at"] = p.PaidAt
	}
	return resp
}

func writePaymentError(c *gin.Context, err error) {
	switch err {
	case payment.ErrBadRequest:
		writeError(c, http.StatusBadRequest, err.Error())
	case payment.ErrNotFound:
		writeError(c, http.StatusNotFound, err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "internal error")
	}
}
