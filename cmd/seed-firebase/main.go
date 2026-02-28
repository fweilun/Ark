// seed-firebase seeds 5 mock drivers near Taipei 101 directly into Firebase
// RTDB so the app shows live vehicles immediately during development.
// The frontend (and the RTDB poller) will pick them up automatically.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
)

const (
	credFile    = "zoozoo-v1-firebase-adminsdk-fbsvc-2adb5592ce.json"
	baseLat     = 25.033964
	baseLng     = 121.564468
	offsetRange = 0.02 // ±0.01 degrees ≈ ±1 km
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	projectID, err := readProjectID(credFile)
	if err != nil {
		log.Fatalf("read credentials: %v", err)
	}

	app, err := firebase.NewApp(ctx, &firebase.Config{
		DatabaseURL: fmt.Sprintf("https://%s-default-rtdb.asia-southeast1.firebasedatabase.app", projectID),
	}, option.WithCredentialsFile(credFile))
	if err != nil {
		log.Fatalf("firebase init: %v", err)
	}

	dbClient, err := app.Database(ctx)
	if err != nil {
		log.Fatalf("RTDB client: %v", err)
	}

	log.Printf("seeding 5 mock drivers near Taipei 101 (%.6f, %.6f)...", baseLat, baseLng)

	for i := 1; i <= 5; i++ {
		id := fmt.Sprintf("seed-driver-%d", i)
		lat := baseLat + (rand.Float64()-0.5)*offsetRange
		lng := baseLng + (rand.Float64()-0.5)*offsetRange

		ref := dbClient.NewRef("driver_locations/" + id)
		err := ref.Set(ctx, map[string]interface{}{
			"lat":       lat,
			"lng":       lng,
			"status":    "online",
			"timestamp": time.Now().UnixMilli(),
		})
		if err != nil {
			log.Printf("  [FAIL] driver %s: %v", id, err)
			continue
		}
		log.Printf("  [OK]   driver %s → (%.6f, %.6f)", id, lat, lng)
	}

	log.Println("done — the RTDB poller will sync these to Redis within 30 s.")
}

func readProjectID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	var sa struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(data, &sa); err != nil {
		return "", fmt.Errorf("parsing %s: %w", path, err)
	}
	if sa.ProjectID == "" {
		return "", fmt.Errorf("project_id is empty in %s", path)
	}
	return sa.ProjectID, nil
}
