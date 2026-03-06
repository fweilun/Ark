package location

import (
	"context"
	"math"
	"os"
	"testing"
	"time"

	"ark/internal/types"
)

// TestService_SyncRTDBToRedis is an integration test that writes a dummy driver
// position to Firebase RTDB, triggers a manual poll, and verifies that the
// record is successfully synced into the local Redis GEO index.
func TestService_SyncRTDBToRedis(t *testing.T) {
	// 1. Initialize local Redis
	rdb := newTestRedis(t)

	// 2. Initialize the Store with real Firebase credentials from env
	credsJSON := os.Getenv("FIREBASE_CREDENTIALS_JSON")
	if credsJSON == "" {
		t.Skip("Skipping: FIREBASE_CREDENTIALS_JSON not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := NewStore(ctx, nil, rdb, []byte(credsJSON))
	if err != nil {
		t.Skipf("Skipping RTDB integration test due to initialization failure (missing creds?): %v", err)
	}

	// 3. Setup test data
	const testDriverID = "test-poller-bot-777"
	const userType = "driver"
	pos := types.Point{Lat: 25.033964, Lng: 121.564468} // Taipei 101

	// Ensure clean slate before test
	cleanupMember(t, rdb, userType, types.ID(testDriverID))
	node, _ := rtdbNodeAndStatus(userType)
	ref := store.dbClient.NewRef(node + "/" + testDriverID)
	_ = ref.Delete(ctx)

	// Set cleanup hook to remove data from both Firebase and Redis after test finishes
	t.Cleanup(func() {
		_ = ref.Delete(context.Background())
		cleanupMember(t, rdb, userType, types.ID(testDriverID))
	})

	// 4. Write dummy driver state directly to Firebase RTDB
	entry := rtdbUserEntry{
		Lat:    pos.Lat,
		Lng:    pos.Lng,
		Status: "online",
	}
	if err := ref.Set(ctx, entry); err != nil {
		t.Fatalf("Failed to write mock data to Firebase RTDB: %v", err)
	}

	// Double check that it's NOT in Redis before we run the poller
	beforeSyncUsers, _ := store.GetNearbyUsersFromRedis(ctx, pos.Lat, pos.Lng, 1.0, userType)
	for _, u := range beforeSyncUsers {
		if u.ID == types.ID(testDriverID) {
			t.Fatalf("Driver %s is already in Redis before sync!", testDriverID)
		}
	}

	// 5. Initialize Service and trigger the manual sync (Polling)
	svc := NewService(store)
	svc.syncRTDBToRedis(ctx)

	// 6. Verify that it was pulled correctly and inserted into Redis GEO
	afterSyncUsers, err := store.GetNearbyUsersFromRedis(ctx, pos.Lat, pos.Lng, 1.0, userType)
	if err != nil {
		t.Fatalf("Failed to query Redis GEO after sync: %v", err)
	}

	found := false
	for _, u := range afterSyncUsers {
		if u.ID == types.ID(testDriverID) {
			found = true
			if math.Abs(u.Lat-pos.Lat) > 0.0001 || math.Abs(u.Lng-pos.Lng) > 0.0001 {
				t.Errorf("Coordinates mismatched! Expected (%f, %f), Got (%f, %f)", pos.Lat, pos.Lng, u.Lat, u.Lng)
			}
			break
		}
	}

	if !found {
		t.Errorf("Expected driver %s to be successfully synced to Redis GEO, but it was not found.", testDriverID)
	}
}
