// README: Notification service with Firebase Cloud Messaging (FCM) integration.
package notification

import (
	"context"
	"log"
	"sync"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"

	"ark/internal/types"
)

// NotificationMessage holds the payload for a push notification.
// Note: only string values in Data are forwarded to Firebase; non-string values are ignored.
type NotificationMessage struct {
	Title string
	Body  string
	// Data contains key-value pairs to include in the notification payload.
	// Only string values are supported; non-string values will be silently ignored.
	Data map[string]interface{}
}

// NotificationService defines operations for device registration and push delivery.
type NotificationService interface {
	// EnsureDevice registers or updates a device FCM token.
	EnsureDevice(ctx context.Context, userID types.ID, token, platform, deviceID string) error

	// NotifyUser sends a push notification to all devices registered for the user.
	NotifyUser(ctx context.Context, userID types.ID, message *NotificationMessage) error

	// DeleteOutdatedDevices removes stale device records (called by a scheduled task).
	DeleteOutdatedDevices(ctx context.Context, before time.Time) error
}

// Service is the concrete implementation of NotificationService.
type Service struct {
	store     NotificationStore
	messaging *messaging.Client
}

// NewService creates a Service backed by store.
// credentialsJSON is optional; if empty, FCM sending is skipped (tokens are still persisted).
func NewService(store NotificationStore, credentialsJSON []byte) (*Service, error) {
	svc := &Service{store: store}
	if len(credentialsJSON) == 0 {
		return svc, nil
	}

	app, err := firebase.NewApp(context.Background(), nil, option.WithCredentialsJSON(credentialsJSON))
	if err != nil {
		return nil, err
	}
	msgClient, err := app.Messaging(context.Background())
	if err != nil {
		return nil, err
	}
	svc.messaging = msgClient
	return svc, nil
}

// EnsureDevice upserts the device token in the store.
func (s *Service) EnsureDevice(ctx context.Context, userID types.ID, token, platform, deviceID string) error {
	return s.store.UpsertDevice(ctx, userID, token, platform, deviceID)
}

// NotifyUser retrieves all FCM tokens for the user and sends the notification
// to each token concurrently. It waits for all goroutines to complete before returning.
func (s *Service) NotifyUser(ctx context.Context, userID types.ID, message *NotificationMessage) error {
	tokens, err := s.store.GetTokensByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if len(tokens) == 0 || s.messaging == nil {
		return nil
	}

	data := make(map[string]string, len(message.Data))
	for k, v := range message.Data {
		if sv, ok := v.(string); ok {
			data[k] = sv
		}
	}

	var wg sync.WaitGroup
	for _, token := range tokens {
		token := token
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Use a background context so that notification sends are not cut short
			// if the caller's request context is canceled after NotifyUser returns.
			_, sendErr := s.messaging.Send(context.Background(), &messaging.Message{
				Token: token,
				Notification: &messaging.Notification{
					Title: message.Title,
					Body:  message.Body,
				},
				Data: data,
			})
			if sendErr != nil {
				// [TODO] Handle stale/unregistered tokens and other send failures.
				// See issue discussion: token cleanup on uninstall/account deletion.
				log.Printf("notification: failed to send to token %s: %v", token, sendErr)
			}
		}()
	}
	wg.Wait()
	return nil
}

// DeleteOutdatedDevices delegates to the store to remove stale device records.
func (s *Service) DeleteOutdatedDevices(ctx context.Context, before time.Time) error {
	return s.store.DeleteOutdatedDevices(ctx, before)
}
