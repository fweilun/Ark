// README: Entry point; loads config, wires services, starts HTTP server and background schedulers.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
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
	"ark/internal/modules/relation"
	"ark/internal/ai"
	"ark/internal/maps"
	"ark/internal/modules/rideassistant"
	"ark/internal/modules/user"
	"ark/internal/worker"
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

	notificationStore := notification.NewStore(dbPool)
	notificationSvc, err := notification.NewService(notificationStore, []byte(cfg.Notification.FirebaseCredentialsJSON))
	if err != nil {
		log.Fatal(err)
	}

	matchingStore := matching.NewStore(redisClient, dbPool)

	locationStore, err := location.NewStore(ctx, dbPool, redisClient, []byte(cfg.Notification.FirebaseCredentialsJSON))
	if err != nil {
		log.Fatal(err)
	}
	locationSvc := location.NewService(locationStore)

	matchingSvc := matching.NewService(matchingStore, orderSvc, notificationSvc, locationSvc, cfg.Matching)

	aiStore := aiusage.NewStore(dbPool)
	aiSvc, err := aiusage.NewService(aiStore, cfg.AI.GeminiKey)
	if err != nil {
		log.Fatal(err)
	}
	defer aiSvc.Close()

	calendarStore := calendar.NewStore(dbPool)
	calendarSvc := calendar.NewService(calendarStore, orderSvc)

	driverStore := driver.NewStore(dbPool)
	driverSvc := driver.NewService(driverStore)
	userStore := user.NewStore(dbPool)
	userSvc := user.NewService(userStore)
	relationStore := relation.NewStore(dbPool)
	relationSvc := relation.NewService(relationStore)
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

	// Ride assistant — wired with Gemini AI, Maps geocoding, and order service.
	raStore := rideassistant.NewStore()
	var raPlanner rideassistant.Planner
	var raGeocoder rideassistant.Geocoder
	raOrderAdapter := rideassistant.NewOrderServiceAdapter(orderSvc)

	geminiProvider, err := ai.NewGeminiProvider(ctx, cfg.AI.GeminiKey)
	if err != nil {
		log.Printf("ride assistant: Gemini init failed, using stub planner: %v", err)
		raPlanner = rideassistant.NewStubPlanner()
	} else {
		raPlanner = rideassistant.NewGeminiAdapter(geminiProvider)
		defer geminiProvider.Close()
	}

	if cfg.AI.MapsAPIKey != "" {
		routeSvc, err := maps.NewRouteService(cfg.AI.MapsAPIKey)
		if err != nil {
			log.Printf("ride assistant: Maps RouteService init failed, geocoding disabled: %v", err)
		} else {
			raGeocoder = rideassistant.NewMapsGeocoder(routeSvc)
		}
	}

	raSvc := rideassistant.NewService(raStore, raPlanner, raOrderAdapter, raGeocoder)

	workerRegistry := worker.NewRegistry()

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
		Relation:     relationSvc,
		Auth:          tokenVerifier,
		RideAssistant: raSvc,
		DB:            dbPool,
		Redis:        redisClient,
		Workers:      workerRegistry,
	})

	server := &http.Server{Addr: cfg.HTTP.Addr, Handler: handler.Routes()}

	restartDelay := 5 * time.Second
	reg := workerRegistry
	go worker.RunWithRecovery(ctx, "rtdb-poller", func(c context.Context) {
		locationSvc.RunRTDBPoller(c, 30*time.Second)
	}, restartDelay, reg)
	go worker.RunWithRecovery(ctx, "matching-scheduler", matchingSvc.RunScheduler, restartDelay, reg)
	go worker.RunWithRecovery(ctx, "notification-scheduler", matchingSvc.RunNotificationScheduler, restartDelay, reg)
	go worker.RunWithRecovery(ctx, "timeout-monitor", orderSvc.RunTimeoutMonitor, restartDelay, reg)
	go worker.RunWithRecovery(ctx, "schedule-incentive", orderSvc.RunScheduleIncentiveTicker, restartDelay, reg)
	go worker.RunWithRecovery(ctx, "schedule-expire", orderSvc.RunScheduleExpireTicker, restartDelay, reg)

	// Start HTTP server in a goroutine.
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	log.Printf("server listening on %s", cfg.HTTP.Addr)

	// Block until shutdown signal.
	<-ctx.Done()
	log.Println("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown error: %v", err)
	}
	log.Println("server stopped gracefully")
}
