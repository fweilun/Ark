package aiusage

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

const (
	geminiModel = "gemini-2.0-flash"
)

// newGeminiModel creates a reusable Gemini client and model for the given API key.
// The caller is responsible for calling client.Close() when done.
func newGeminiModel(ctx context.Context, apiKey string) (*genai.Client, *genai.GenerativeModel, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, nil, fmt.Errorf("gemini: missing api key")
	}
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, nil, fmt.Errorf("gemini: create client: %w", err)
	}
	return client, client.GenerativeModel(geminiModel), nil
}

// generateText sends message to the provided Gemini model and returns the reply text.
func generateText(ctx context.Context, model *genai.GenerativeModel, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("gemini: empty message")
	}

	resp, err := model.GenerateContent(ctx, genai.Text(message))
	if err != nil {
		return "", fmt.Errorf("gemini: generate content: %w", err)
	}
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", fmt.Errorf("gemini: API returned empty candidates")
	}

	var textParts []string
	for _, part := range resp.Candidates[0].Content.Parts {
		txt, ok := part.(genai.Text)
		if !ok || strings.TrimSpace(string(txt)) == "" {
			continue
		}
		textParts = append(textParts, string(txt))
	}
	if len(textParts) == 0 {
		return "", fmt.Errorf("gemini: API returned empty text parts")
	}

	return strings.Join(textParts, "\n"), nil
}
