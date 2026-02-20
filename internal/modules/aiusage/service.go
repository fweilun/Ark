package aiusage

import "context"

// Service orchestrates AI token-usage logic.
type Service struct {
	store     *Store
	openAIKey string
}

// NewService creates a Service backed by the given Store.
// openAIKey is the OpenAI API key used for ChatGPT requests.
func NewService(store *Store, openAIKey string) *Service {
	return &Service{store: store, openAIKey: openAIKey}
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

// Chat deducts one token from uid's monthly quota and calls ChatGPT with the given message.
// Returns ErrInsufficientTokens if the quota is exhausted before making the API call.
func (s *Service) Chat(ctx context.Context, uid, message string) (string, error) {
	if err := s.UseToken(ctx, uid); err != nil {
		return "", err
	}
	return callChatGPT(ctx, s.openAIKey, message)
}
