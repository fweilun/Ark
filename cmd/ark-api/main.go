// README: Entry point; loads config, wires services, starts HTTP server and background schedulers.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"

	"ark/internal/config"
	httptransport "ark/internal/http"
	"ark/internal/http/middleware"
	"ark/internal/infra"
	"ark/internal/modules/aiusage"
	"ark/internal/modules/calendar"
	"ark/internal/modules/driver"
	"ark/internal/modules/location"
	"ark/internal/modules/matching"
	"ark/internal/modules/notification"
	"ark/internal/modules/order"
	"ark/internal/modules/pricing"
	"ark/internal/modules/user"
	"ark/internal/types"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbPool, err := infra.NewDB(ctx, cfg.DB.DSN)
	if err != nil {
		log.Fatal(err)
	}

	redisClient := infra.NewRedis(cfg.Redis.Addr)

	pricingStore := pricing.NewStore(dbPool)
	pricingSvc := pricing.NewService(pricingStore)

	orderStore := order.NewStore(dbPool)
	orderSvc := order.NewService(orderStore, pricingSvc)

	matchingStore := matching.NewStore(redisClient)
	matchingSvc := matching.NewService(matchingStore, orderSvc, cfg.Matching)

	locationStore := location.NewStore(dbPool, redisClient)
	locationSvc := location.NewService(locationStore)

	aiStore := aiusage.NewStore(dbPool)
	aiSvc, err := aiusage.NewService(aiStore, cfg.AI.GeminiKey)
	if err != nil {
		log.Fatal(err)
	}
	defer aiSvc.Close()

	notificationStore := notification.NewStore(dbPool)
	notificationSvc, err := notification.NewService(notificationStore, []byte(cfg.Notification.FirebaseCredentialsJSON))
	if err != nil {
		log.Fatal(err)
	}

	calendarStore := calendar.NewStore(dbPool)
	calendarSvc := calendar.NewService(calendarStore, orderSvc)

	driverStore := driver.NewStore(dbPool)
	driverSvc := driver.NewService(driverStore)
	userStore := user.NewStore(dbPool)
	userSvc := user.NewService(userStore)
	// Initialize Firebase auth client for token verification.
	// If FIREBASE_CREDENTIALS_JSON is not set, auth middleware is disabled (dev mode).
	var tokenVerifier middleware.TokenVerifier
	if creds := cfg.Notification.FirebaseCredentialsJSON; creds != "" {
		fbApp, err := firebase.NewApp(ctx, nil, option.WithCredentialsJSON([]byte(creds)))
		if err != nil {
			log.Fatalf("initialising Firebase app for auth: %v", err)
		}
		authClient, err := fbApp.Auth(ctx)
		if err != nil {
			log.Fatalf("initialising Firebase auth client: %v", err)
		}
		tokenVerifier = authClient
	} else {
		log.Printf("SECURITY WARNING: FIREBASE_CREDENTIALS_JSON not set; auth middleware disabled (dev mode)")
	}

	handler := httptransport.NewServer(httptransport.ServerDeps{
		Order:        orderSvc,
		Matching:     matchingSvc,
		Location:     locationSvc,
		Pricing:      pricingSvc,
		AI:           aiSvc,
		Notification: notificationSvc,
		Calendar:     calendarSvc,
		Driver:       driverSvc,
		User:         userSvc,
		Auth:         tokenVerifier,
	})

	server := &http.Server{Addr: cfg.HTTP.Addr, Handler: handler.Routes()}

	go matchingSvc.RunScheduler(ctx)
	go orderSvc.RunTimeoutMonitor(ctx)
	go orderSvc.RunScheduleIncentiveTicker(ctx)
	go orderSvc.RunScheduleExpireTicker(ctx)

	// Reuse the same Firebase service-account credentials for location sync;
	// both notification and location operate on the same Firebase project.
	if creds := cfg.Notification.FirebaseCredentialsJSON; creds != "" {
		fbLocSvc, err := location.NewFirebaseServiceFromJSON(ctx, []byte(creds))
		if err != nil {
			log.Printf("WARNING: could not init Firebase location service for sync: %v", err)
		} else {
			interval := time.Duration(cfg.Location.SyncIntervalSeconds) * time.Second
			go runLocationSync(ctx, fbLocSvc, locationSvc, interval)
		}
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// runLocationSync polls Firebase RTDB every interval and updates the Redis GEO
// cache via the location service. The interval is configurable via
// ARK_LOCATION_SYNC_INTERVAL (default 60 s).
func runLocationSync(ctx context.Context, fbSvc *location.FirebaseService, svc *location.Service, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			syncLocations(ctx, fbSvc, svc)
		}
	}
}

func syncLocations(ctx context.Context, fbSvc *location.FirebaseService, svc *location.Service) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		syncUserType(ctx, fbSvc.FetchAllDrivers, svc, "driver")
	}()
	go func() {
		defer wg.Done()
		syncUserType(ctx, fbSvc.FetchAllPassengers, svc, "passenger")
	}()
	wg.Wait()
}

func syncUserType(
	ctx context.Context,
	fetch func(context.Context) (map[types.ID]types.Point, error),
	svc *location.Service,
	userType string,
) {
	positions, err := fetch(ctx)
	if err != nil {
		log.Printf("location sync: fetch %s locations: %v", userType, err)
		return
	}
	for id, pos := range positions {
		if err := svc.Update(ctx, location.Update{UserID: id, UserType: userType, Position: pos}); err != nil {
			log.Printf("location sync: update %s %s: %v", userType, id, err)
		}
	}
}
