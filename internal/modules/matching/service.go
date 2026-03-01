// README: Matching service orchestrates candidate pools and triggers order matching.
package matching

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"strconv"
	"time"

	"ark/internal/config"
	"ark/internal/modules/location"
	"ark/internal/modules/notification"
	"ark/internal/modules/order"
	"ark/internal/types"
)

const (
	// notificationCooldown is the minimum interval between successive notifications
	// for the same order to avoid driver spam.
	notificationCooldown = 5 * time.Minute
	// maxNotifyDrivers is the maximum number of drivers to notify per cycle.
	maxNotifyDrivers = 5
)

type OrderMatcher interface {
	Match(ctx context.Context, cmd order.MatchCommand) error
}

// DriverLocator provides access to online driver location data.
type DriverLocator interface {
	GetAllDrivers(ctx context.Context) ([]location.DriverLocation, error)
}

type Service struct {
	store        *Store
	order        OrderMatcher
	notification notification.NotificationService
	location     DriverLocator
	cfg          config.MatchingConfig
}

func NewService(
	store *Store,
	order OrderMatcher,
	notif notification.NotificationService,
	loc DriverLocator,
	cfg config.MatchingConfig,
) *Service {
	return &Service{
		store:        store,
		order:        order,
		notification: notif,
		location:     loc,
		cfg:          cfg,
	}
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
	tick := time.Duration(s.cfg.TickSeconds) * time.Second
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

// RunNotificationScheduler periodically finds the most urgent unmatched order and
// broadcasts it to a random selection of online drivers via push notification.
// The cooldown between notifications for the same order is notificationCooldown.
func (s *Service) RunNotificationScheduler(ctx context.Context) {
	tick := time.Duration(s.cfg.TickSeconds) * time.Second
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.notifyMostUrgentOrder(ctx); err != nil {
				log.Printf("matching: notification scheduler error: %v", err)
			}
		}
	}
}

// notifyMostUrgentOrder finds the most urgent unmatched order not in cooldown,
// selects up to maxNotifyDrivers random online drivers, sends push notifications,
// and records the attempt with a cooldown timestamp.
func (s *Service) notifyMostUrgentOrder(ctx context.Context) error {
	// 1. Get the most urgent order not in cooldown.
	urgentOrder, existingNotif, err := s.store.GetMostUrgentNotifiable(ctx)
	if err != nil {
		return err
	}
	if urgentOrder == nil {
		return nil // no eligible orders
	}

	// 2. Get all online drivers.
	if s.location == nil {
		return nil
	}
	drivers, err := s.location.GetAllDrivers(ctx)
	if err != nil {
		return err
	}
	if len(drivers) == 0 {
		return nil
	}

	// 3. Randomly select up to maxNotifyDrivers drivers.
	selected := pickRandom(drivers, maxNotifyDrivers)

	// 4. Push notification to each selected driver.
	msg := buildOrderNotificationMessage(urgentOrder)
	for _, d := range selected {
		if err := s.notification.NotifyUser(ctx, d.DriverID, msg); err != nil {
			log.Printf("matching: failed to notify driver %s for order %s: %v", d.DriverID, urgentOrder.ID, err)
		}
	}

	// 5. Mark the order as notified and set the next cooldown window.
	notifyCount := 1
	if existingNotif != nil {
		notifyCount = existingNotif.NotifyCount + 1
	}
	now := time.Now()
	return s.store.UpsertOrderNotification(ctx, &OrderNotification{
		OrderID:          urgentOrder.ID,
		NotifyCount:      notifyCount,
		LastNotifiedAt:   now,
		NextNotifiableAt: now.Add(notificationCooldown),
	})
}

// pickRandom returns up to n randomly selected elements from drivers.
func pickRandom(drivers []location.DriverLocation, n int) []location.DriverLocation {
	if len(drivers) <= n {
		return drivers
	}
	perm := rand.Perm(len(drivers))
	result := make([]location.DriverLocation, n)
	for i := 0; i < n; i++ {
		result[i] = drivers[perm[i]]
	}
	return result
}

// buildOrderNotificationMessage creates a push notification payload for the given order.
func buildOrderNotificationMessage(o *order.Order) *notification.NotificationMessage {
	return &notification.NotificationMessage{
		Title: "New ride request",
		Body:  "A passenger needs a driver. Tap to view details.",
		Data: map[string]interface{}{
			"type":        "order_notification",
			"order_id":    string(o.ID),
			"pickup_lat":  strconv.FormatFloat(o.Pickup.Lat, 'f', 6, 64),
			"pickup_lng":  strconv.FormatFloat(o.Pickup.Lng, 'f', 6, 64),
			"dropoff_lat": strconv.FormatFloat(o.Dropoff.Lat, 'f', 6, 64),
			"dropoff_lng": strconv.FormatFloat(o.Dropoff.Lng, 'f', 6, 64),
			"order_type":  o.OrderType,
		},
	}
}
