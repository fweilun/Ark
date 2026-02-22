// README: AI chat handler (token-guarded Gemini chat).
package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"ark/internal/modules/aiusage"
)

type AIHandler struct {
	ai *aiusage.Service
}

func NewAIHandler(aiSvc *aiusage.Service) *AIHandler {
	return &AIHandler{ai: aiSvc}
}

type aiChatReq struct {
	UID     string `json:"uid"`
	Message string `json:"message"`
}

// Chat handles POST /api/ai/chat.
func (h *AIHandler) Chat(c *gin.Context) {
	var req aiChatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "invalid json")
		return
	}

	req.UID = strings.TrimSpace(req.UID)
	req.Message = strings.TrimSpace(req.Message)
	if req.UID == "" || req.Message == "" {
		writeError(c, http.StatusBadRequest, "missing uid or message")
		return
	}
	if !isValidID(req.UID) {
		writeError(c, http.StatusBadRequest, "invalid uid")
		return
	}

	context, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	reply, err := h.ai.Chat(context, req.UID, req.Message)
	if err != nil {
		switch {
		case errors.Is(err, aiusage.ErrInsufficientTokens):
			writeError(c, http.StatusTooManyRequests, err.Error())
		default:
			writeError(c, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(c, http.StatusOK, map[string]any{"reply": reply})
}
