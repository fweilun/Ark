package aiusage

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
)

// Service orchestrates AI token-usage logic.
type Service struct {
	store  *Store
	client *genai.Client
	model  *genai.GenerativeModel
}

// NewService creates a Service backed by the given Store.
// If geminiKey is non-empty, a long-lived Gemini client is initialized immediately.
// Call Close() to release Gemini client resources when the Service is no longer needed.
func NewService(store *Store, geminiKey string) (*Service, error) {
	svc := &Service{store: store}
	if geminiKey == "" {
		return svc, nil
	}
	client, model, err := newGeminiModel(context.Background(), geminiKey)
	if err != nil {
		return nil, err
	}
	svc.client = client
	svc.model = model
	return svc, nil
}

// Close releases the long-lived Gemini client resources.
func (s *Service) Close() {
	if s.client != nil {
		s.client.Close()
	}
}

// UseToken deducts one token from the user's monthly allowance.
// If the user row does not exist yet it is initialised and the token is immediately consumed.
// Returns ErrInsufficientTokens when the quota for the current month is exhausted.
func (s *Service) UseToken(ctx context.Context, uid string) error {
	err := s.store.UseToken(ctx, uid)
	if err != ErrInsufficientTokens {
		return err
	}

	// RowsAffected == 0: row missing OR quota exhausted.
	// Only retry if a new row was actually inserted (missing-row case).
	created, initErr := s.store.EnsureUser(ctx, uid)
	if initErr != nil {
		return initErr
	}
	if !created {
		// User exists but quota is exhausted for this month.
		return ErrInsufficientTokens
	}
	return s.store.UseToken(ctx, uid)
}

// Chat deducts one token from uid's monthly quota and calls Gemini with the given message.
// Returns ErrInsufficientTokens if the quota is exhausted before making the API call.
// Returns an error if the Gemini client was not initialized (empty geminiKey at construction).
func (s *Service) Chat(ctx context.Context, uid, message string) (string, error) {
	if s.model == nil {
		return "", fmt.Errorf("gemini: client not initialized (empty api key)")
	}
	if err := s.UseToken(ctx, uid); err != nil {
		return "", err
	}
	return generateText(ctx, s.model, message)
}
