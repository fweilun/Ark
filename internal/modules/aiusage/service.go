package aiusage

import (
	"context"
	"fmt"

	"ark/internal/modules/planner"

	"github.com/google/generative-ai-go/genai"
)

// Service is the public interface for the aiusage business logic.
type Service interface {
	// UseToken deducts one token from the user's monthly allowance.
	UseToken(ctx context.Context, uid string) error

	// Chat deducts one token and generates a plain text AI reply.
	Chat(ctx context.Context, uid, message string) (string, error)

	// ParseIntent deducts one token and calls the AI to interpret the user's message.
	ParseIntent(ctx context.Context, uid, message string, contextMap map[string]string) (*IntentResult, error)

	// Close releases underlying AI client resources.
	Close()
}

// OrderService defines the subset of order operations the AI secretary needs.
// Declared here to avoid an import cycle with the order package.
type OrderService interface {
	GetOrder(ctx context.Context, id string) (interface{}, error)
	CancelOrder(ctx context.Context, id, actorType, reason string) error
}

// aiusageService is the private implementation of Service.
type aiusageService struct {
	store          *Store
	aiClient       AIClient
	plannerService planner.Service
	orderService   OrderService

	// Legacy plain-text chat path.
	chatModel *genai.GenerativeModel
	rawClient *genai.Client
}

// ServiceConfig holds all dependencies for constructing an aiusageService.
type ServiceConfig struct {
	Store          *Store
	AIClient       AIClient
	PlannerService planner.Service
	OrderService   OrderService
	// GeminiKey enables the legacy Chat() plain-text path. Leave empty to disable.
	GeminiKey string
}

// NewService constructs the Service with all dependencies injected.
func NewService(cfg ServiceConfig) (Service, error) {
	svc := &aiusageService{
		store:          cfg.Store,
		aiClient:       cfg.AIClient,
		plannerService: cfg.PlannerService,
		orderService:   cfg.OrderService,
	}
	if cfg.GeminiKey != "" {
		c, m, err := newGeminiModel(context.Background(), cfg.GeminiKey)
		if err != nil {
			return nil, err
		}
		svc.rawClient = c
		svc.chatModel = m
	}
	return svc, nil
}

// Close releases AI client and raw Gemini resources.
func (s *aiusageService) Close() {
	if s.aiClient != nil {
		s.aiClient.Close()
	}
	if s.rawClient != nil {
		s.rawClient.Close()
	}
}

// UseToken deducts one token from the user's monthly allowance.
func (s *aiusageService) UseToken(ctx context.Context, uid string) error {
	err := s.store.UseToken(ctx, uid)
	if err != ErrInsufficientTokens {
		return err
	}
	created, initErr := s.store.EnsureUser(ctx, uid)
	if initErr != nil {
		return initErr
	}
	if !created {
		return ErrInsufficientTokens
	}
	return s.store.UseToken(ctx, uid)
}

// Chat deducts one token and returns a plain-text Gemini reply.
func (s *aiusageService) Chat(ctx context.Context, uid, message string) (string, error) {
	if s.chatModel == nil {
		return "", fmt.Errorf("gemini: chat client not initialized (empty api key)")
	}
	if err := s.UseToken(ctx, uid); err != nil {
		return "", err
	}
	return generateText(ctx, s.chatModel, message)
}

// ParseIntent deducts one token and calls the AI to interpret the user's message.
func (s *aiusageService) ParseIntent(ctx context.Context, uid, message string, contextMap map[string]string) (*IntentResult, error) {
	if s.aiClient == nil {
		return nil, fmt.Errorf("aiusage: AI client not initialized")
	}
	if err := s.UseToken(ctx, uid); err != nil {
		return nil, err
	}
	return s.aiClient.ParseUserIntent(ctx, message, contextMap)
}
