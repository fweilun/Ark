// README: Entry point; loads config, wires services, starts HTTP server and background schedulers.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"ark/internal/config"
	httptransport "ark/internal/http"
	"ark/internal/infra"
	"ark/internal/modules/aiusage"
	"ark/internal/modules/calendar"
	"ark/internal/modules/location"
	"ark/internal/modules/matching"
	"ark/internal/modules/notification"
	"ark/internal/modules/order"
	"ark/internal/modules/pricing"
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

	firebaseSvc, err := location.NewFirebaseService(ctx)
	if err != nil {
		log.Fatal(err)
	}

	locationStore := location.NewStore(dbPool, redisClient)
	locationSvc := location.NewService(locationStore, firebaseSvc)

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

	handler := httptransport.NewServer(httptransport.ServerDeps{
		Order:        orderSvc,
		Matching:     matchingSvc,
		Location:     locationSvc,
		Pricing:      pricingSvc,
		AI:           aiSvc,
		Notification: notificationSvc,
		Calendar:     calendarSvc,
	})

	server := &http.Server{Addr: cfg.HTTP.Addr, Handler: handler.Routes()}

	go matchingSvc.RunScheduler(ctx)
	go orderSvc.RunTimeoutMonitor(ctx)
	go orderSvc.RunScheduleIncentiveTicker(ctx)
	go orderSvc.RunScheduleExpireTicker(ctx)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
