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

// callGemini sends message to Gemini via Google's official SDK and returns the reply text.
func callGemini(ctx context.Context, apiKey, message string) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("gemini: missing api key")
	}
	if strings.TrimSpace(message) == "" {
		return "", fmt.Errorf("gemini: empty message")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("gemini: create client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel(geminiModel)

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
