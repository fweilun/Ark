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
	"ark/internal/modules/user"
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

	locationStore, err := location.NewStore(ctx, dbPool, redisClient)
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
		Auth:         tokenVerifier,
	})

	server := &http.Server{Addr: cfg.HTTP.Addr, Handler: handler.Routes()}

	go locationSvc.RunRTDBPoller(ctx, 30*time.Second)
	go matchingSvc.RunScheduler(ctx)
	go matchingSvc.RunNotificationScheduler(ctx)
	go orderSvc.RunTimeoutMonitor(ctx)
	go orderSvc.RunScheduleIncentiveTicker(ctx)
	go orderSvc.RunScheduleExpireTicker(ctx)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
