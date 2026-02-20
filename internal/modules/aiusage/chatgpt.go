package aiusage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const openAIEndpoint = "https://api.openai.com/v1/chat/completions"

// httpClient is used for all ChatGPT requests; the 30s timeout guards against stalled connections
// while context cancellation is still honoured via NewRequestWithContext.
var httpClient = &http.Client{Timeout: 30 * time.Second}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// callChatGPT sends message to the OpenAI chat completions endpoint and returns the reply text.
func callChatGPT(ctx context.Context, apiKey, message string) (string, error) {
	reqBody, err := json.Marshal(chatRequest{
		Model:    "gpt-3.5-turbo",
		Messages: []chatMessage{{Role: "user", Content: message}},
	})
	if err != nil {
		return "", fmt.Errorf("chatgpt: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIEndpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("chatgpt: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("chatgpt: do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("chatgpt: read response: %w", err)
	}

	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", fmt.Errorf("chatgpt: unmarshal response: %w", err)
	}
	if cr.Error != nil {
		return "", fmt.Errorf("chatgpt: api error: %s", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("chatgpt: API returned empty choices array (raw: %s)", body)
	}
	return cr.Choices[0].Message.Content, nil
}
