// seed-firebase seeds 5 mock drivers near Taipei 101 into Firebase RTDB so
// the app shows live vehicles immediately during development.
package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"

	"ark/internal/modules/location"
	"ark/internal/types"
)

const (
	baseLat     = 25.033964
	baseLng     = 121.564468
	offsetRange = 0.02 // ±0.01 degrees ≈ ±1 km
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Println("initialising location store (Firebase)...")
	// db and redis are nil; only Firebase write methods are called below.
	store, err := location.NewStore(ctx, nil, nil)
	if err != nil {
		log.Fatalf("NewStore: %v", err)
	}

	log.Printf("seeding 5 mock drivers near Taipei 101 (%.6f, %.6f)...", baseLat, baseLng)

	for i := 1; i <= 5; i++ {
		id := types.ID(fmt.Sprintf("seed-driver-%d", i))
		pos := types.Point{
			Lat: baseLat + (rand.Float64()-0.5)*offsetRange,
			Lng: baseLng + (rand.Float64()-0.5)*offsetRange,
		}

		if err := store.WriteLocation(ctx, id, pos, "driver"); err != nil {
			log.Printf("  [FAIL] driver %s: %v", id, err)
			continue
		}
		log.Printf("  [OK]   driver %s → (%.6f, %.6f)", id, pos.Lat, pos.Lng)
	}

	log.Println("done — open the app to see the seeded drivers.")
}
