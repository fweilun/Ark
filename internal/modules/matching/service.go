// README: Matching service orchestrates candidate pools and triggers order matching.
package matching

import (
    "context"
    "errors"
    "time"

    "ark/internal/config"
    "ark/internal/modules/order"
    "ark/internal/types"
)

type OrderMatcher interface {
    Match(ctx context.Context, cmd order.MatchCommand) error
}

type Service struct {
    store   *Store
    order   OrderMatcher
    cfg     config.MatchingConfig
}

func NewService(store *Store, order OrderMatcher, cfg config.MatchingConfig) *Service {
    return &Service{store: store, order: order, cfg: cfg}
}

func (s *Service) AddCandidate(ctx context.Context, c Candidate) error {
    return errors.New("not implemented")
}

func (s *Service) RemoveCandidate(ctx context.Context, id types.ID, t CandidateType) error {
    return errors.New("not implemented")
}

func (s *Service) TryImmediateMatch(ctx context.Context, c Candidate) error {
    return errors.New("not implemented")
}

func (s *Service) RunScheduler(ctx context.Context) {
    tick := time.Duration(s.cfg.Matching.TickSeconds) * time.Second
    ticker := time.NewTicker(tick)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // TODO: pull candidates, query nearby drivers, create matches
        }
    }
}
