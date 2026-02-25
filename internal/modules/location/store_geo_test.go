package location

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"ark/internal/types"
)

// newTestRedis connects to a local Redis instance and skips the test if it is
// unavailable. The connection is closed automatically via t.Cleanup.
func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available at localhost:6379 (%v); skipping Redis GEO tests", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// newTestStore builds a Store backed only by the given Redis client.
// db is nil because these tests do not touch Postgres.
func newTestStore(rdb *redis.Client) *Store {
	return &Store{db: nil, redis: rdb}
}

// cleanupMember removes a GEO member and its status key so each test starts
// from a clean state.
func cleanupMember(t *testing.T, rdb *redis.Client, userType string, id types.ID) {
	t.Helper()
	ctx := context.Background()
	_ = rdb.ZRem(ctx, geoSetKey(userType), string(id))
	_ = rdb.Del(ctx, statusKey(userType, id))
}

// TestSetGeoAndGetNearbyUsersFromRedis verifies that a position written via
// SetGeo is returned by GetNearbyUsersFromRedis when inside the search radius
// and is not returned when the search radius excludes it.
func TestSetGeoAndGetNearbyUsersFromRedis(t *testing.T) {
	rdb := newTestRedis(t)
	store := newTestStore(rdb)
	ctx := context.Background()

	const userType = "driver"
	id := types.ID("test-geo-driver-write-1")
	pos := types.Point{Lat: 25.033964, Lng: 121.564468} // Taipei 101

	cleanupMember(t, rdb, userType, id)
	t.Cleanup(func() { cleanupMember(t, rdb, userType, id) })

	if err := store.SetGeo(ctx, id, pos, userType); err != nil {
		t.Fatalf("SetGeo: %v", err)
	}

	// Within 5 km — driver must be present.
	users, err := store.GetNearbyUsersFromRedis(ctx, 25.033, 121.564, 5.0, userType)
	if err != nil {
		t.Fatalf("GetNearbyUsersFromRedis (within radius): %v", err)
	}
	found := false
	for _, u := range users {
		if u.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected driver %s in results (within radius), got %v", id, users)
	}

	// 500 km away — driver must be absent.
	farUsers, err := store.GetNearbyUsersFromRedis(ctx, 22.0, 114.0, 5.0, userType)
	if err != nil {
		t.Fatalf("GetNearbyUsersFromRedis (outside radius): %v", err)
	}
	for _, u := range farUsers {
		if u.ID == id {
			t.Errorf("driver %s should not appear when search origin is far away", id)
		}
	}
}

// TestGetNearbyUsersFromRedis_LazyDeletion verifies that:
//  1. A user whose status key has expired is filtered out of query results.
//  2. The lazy deletion goroutine removes the stale member from the GEO set.
func TestGetNearbyUsersFromRedis_LazyDeletion(t *testing.T) {
	rdb := newTestRedis(t)
	store := newTestStore(rdb)
	ctx := context.Background()

	const userType = "driver"
	id := types.ID("test-geo-driver-lazy-1")
	pos := types.Point{Lat: 25.033964, Lng: 121.564468}

	cleanupMember(t, rdb, userType, id)
	t.Cleanup(func() { cleanupMember(t, rdb, userType, id) })

	if err := store.SetGeo(ctx, id, pos, userType); err != nil {
		t.Fatalf("SetGeo: %v", err)
	}

	// Simulate TTL expiry by deleting the status key directly.
	if err := rdb.Del(ctx, statusKey(userType, id)).Err(); err != nil {
		t.Fatalf("Del status key: %v", err)
	}

	// Query — the expired user must be filtered out.
	users, err := store.GetNearbyUsersFromRedis(ctx, 25.033, 121.564, 5.0, userType)
	if err != nil {
		t.Fatalf("GetNearbyUsersFromRedis after expiry: %v", err)
	}
	for _, u := range users {
		if u.ID == id {
			t.Errorf("expired driver %s should not appear in results", id)
		}
	}

	// Give the lazy-deletion goroutine time to execute.
	time.Sleep(100 * time.Millisecond)

	// The GEO member must have been removed by ZRem.
	_, err = rdb.ZScore(ctx, geoSetKey(userType), string(id)).Result()
	if !errors.Is(err, redis.Nil) {
		t.Errorf("expected driver %s to be removed from GEO set by lazy deletion; ZScore err: %v", id, err)
	}
}
