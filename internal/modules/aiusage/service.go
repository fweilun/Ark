package aiusage

import "context"

// Service orchestrates AI token-usage logic.
type Service struct {
	store *Store
}

// NewService creates a Service backed by the given Store.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// UseToken deducts one token from the user's monthly allowance.
// If the user row does not exist yet it is initialised and the token is immediately consumed.
// Returns ErrInsufficientTokens when the quota for the current month is exhausted.
func (s *Service) UseToken(ctx context.Context, uid string) error {
	err := s.store.UseToken(ctx, uid)
	if err != ErrInsufficientTokens {
		return err
	}

	// Row may be missing: try to create it, then retry the deduction once.
	if initErr := s.store.EnsureUser(ctx, uid); initErr != nil {
		return initErr
	}
	return s.store.UseToken(ctx, uid)
}
