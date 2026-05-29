// README: AI chat handler (token-guarded Gemini chat).
package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"ark/internal/http/dto"
	"ark/internal/httpx"
	"ark/internal/modules/aiusage"
)

type AIHandler struct {
	ai aiusage.Service
}

func NewAIHandler(aiSvc aiusage.Service) *AIHandler {
	return &AIHandler{ai: aiSvc}
}

type aiChatReq struct {
	UID     string `json:"uid" binding:"required"`
	Message string `json:"message" binding:"required"`
}

// Chat handles POST /api/ai/chat.
//
// @Summary      AI chat (Gemini, token-metered)
// @Description  Sends a message to the Gemini-backed chat service on behalf of `uid`. Requests are rate-limited by the shared token budget; 429 is returned when the caller has exhausted their quota.
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     FirebaseAuth
// @Param        body  body      aiChatReq           true  "User id and message"
// @Success      200   {object}  dto.AIChatResponse
// @Failure      400   {object}  httpx.ErrorBody
// @Failure      429   {object}  httpx.ErrorBody
// @Failure      500   {object}  httpx.ErrorBody
// @Router       /api/ai/chat [post]
func (h *AIHandler) Chat(c *gin.Context) {
	var req aiChatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	req.UID = strings.TrimSpace(req.UID)
	req.Message = strings.TrimSpace(req.Message)
	if req.UID == "" || req.Message == "" {
		httpx.RespondError(c, http.StatusBadRequest, "missing uid or message")
		return
	}
	if !isValidID(req.UID) {
		httpx.RespondError(c, http.StatusBadRequest, "invalid uid")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	reply, err := h.ai.Chat(ctx, req.UID, req.Message)
	if err != nil {
		switch {
		case errors.Is(err, aiusage.ErrInsufficientTokens):
			httpx.RespondError(c, http.StatusTooManyRequests, err.Error())
		default:
			httpx.RespondError(c, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(c, http.StatusOK, dto.AIChatResponse{Reply: reply})
}
