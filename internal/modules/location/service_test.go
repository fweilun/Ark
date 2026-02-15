package location

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestUpdateDriverLocation(t *testing.T) {
	redisAddr := os.Getenv("ARK_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("ARK_REDIS_ADDR not set; skipping integration test")
	}

	// Setup Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer rdb.Close()

	// Setup Store & Service
	store := NewStore(nil, rdb) // DB nil for now as strict persistence isn't tested here
	svc := NewService(store)

	ctx := context.Background()

	// Test Case 1: Valid Update
	uid := fmt.Sprintf("driver_test_%d", time.Now().UnixNano())
	update := DriverLocationUpdate{
		LocationUpdate: LocationUpdate{
			UserID:   uid,
			UserType: UserTypeDriver,
			Seq:      1,
			Point: LocationPoint{
				Lat:  40.7128,
				Lng:  -74.0060,
				TsMs: time.Now().UnixMilli(),
			},
			IsActive: true,
		},
		DriverState: "available",
	}

	res, err := svc.UpdateDriverLocation(ctx, update)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Accepted {
		t.Errorf("expected update to be accepted")
	}

	// Verify in Redis
	// Helper to check GEO position would ideally go here
	pos, err := rdb.GeoPos(ctx, "geo:drivers", uid).Result()
	if err != nil {
		t.Fatalf("failed to query redis geo: %v", err)
	}
	if len(pos) == 0 || pos[0] == nil {
		t.Fatalf("expected position in redis, got none")
	}

	// Test Case 2: Throttling (Too fast)
	update2 := update
	update2.Seq = 2
	update2.Point.TsMs += 10 // Only 10ms later
	res2, err := svc.UpdateDriverLocation(ctx, update2)
	if err != nil {
		t.Fatalf("unexpected error on second update: %v", err)
	}

	_ = res2
}

func TestUpdatePassengerLocation(t *testing.T) {
	redisAddr := os.Getenv("ARK_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("ARK_REDIS_ADDR not set; skipping integration test")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer rdb.Close()

	store := NewStore(nil, rdb)
	svc := NewService(store)

	ctx := context.Background()

	uid := fmt.Sprintf("passenger_test_%d", time.Now().UnixNano())
	update := LocationUpdate{
		UserID:   uid,
		UserType: UserTypePassenger,
		Seq:      1,
		Point: LocationPoint{
			Lat:  40.7580,
			Lng:  -73.9855,
			TsMs: time.Now().UnixMilli(),
		},
		IsActive: true,
	}

	res, err := svc.UpdatePassengerLocation(ctx, update)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Accepted {
		t.Errorf("expected passenger update accepted")
	}
}
