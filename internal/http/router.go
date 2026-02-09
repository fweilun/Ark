// README: HTTP router registration.
package http

import (
    "net/http"

    "ark/internal/http/handlers"
    "ark/internal/modules/location"
    "ark/internal/modules/matching"
    "ark/internal/modules/order"
    "ark/internal/modules/pricing"
)

func NewRouter(
    orderService *order.Service,
    matchingService *matching.Service,
    locationService *location.Service,
    pricingService *pricing.Service,
) http.Handler {
    mux := http.NewServeMux()

    orderHandler := handlers.NewOrderHandler(orderService)
    mux.HandleFunc("POST /api/orders", orderHandler.Create)
    mux.HandleFunc("GET /api/orders/{id}", orderHandler.Get)
    mux.HandleFunc("POST /api/orders/{id}/cancel", orderHandler.Cancel)

    driverHandler := handlers.NewDriverHandler(orderService, matchingService)
    mux.HandleFunc("GET /api/drivers/orders", driverHandler.ListAvailable)
    mux.HandleFunc("POST /api/drivers/orders/{id}/accept", driverHandler.Accept)
    mux.HandleFunc("POST /api/drivers/orders/{id}/start", driverHandler.Start)

    locationHandler := handlers.NewLocationHandler(locationService)
    mux.HandleFunc("PUT /api/drivers/{id}/location", locationHandler.Update)

    passengerHandler := handlers.NewPassengerHandler(orderService)
    mux.HandleFunc("POST /api/passengers/orders", passengerHandler.RequestRide)
    mux.HandleFunc("GET /api/passengers/orders/{id}", passengerHandler.GetOrder)

    mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte("OK"))
    })

    return mux
}
