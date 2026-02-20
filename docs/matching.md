# Matching: Driver Dispatch Algorithm for Scheduled Orders

## Overview

This document describes how the Ark platform matches drivers to scheduled orders.
The implementation lives in `internal/modules/matching/`.

## State Flow

```
Passenger creates scheduled order
        |
        v
  StatusScheduled  ──(T0)──► Matching service dispatches to 5 drivers
        |
        |  (no claim within 30 s)
        v
  Public scheduled list open to all drivers  ──(T-24h)──► Wider broadcast
        |
        |  (driver claims)
        v
  StatusAssigned ──► Driver departs ──► StatusApproaching ──► …
```

## Timing Rules

| Phase   | Trigger                                       | Action                                              |
|---------|-----------------------------------------------|-----------------------------------------------------|
| **T0**  | Scheduler first sees a new scheduled order    | Pick 10 random nearby drivers, notify 5             |
| **T30s**| Order still unclaimed 30 seconds after T0     | Mark order as broadcast; visible to all drivers     |
| **T-24h**| Order unclaimed and ≤ 24 hours until pickup  | Notify up to 10 additional drivers in a wider radius|

## Algorithm: `PickRandomDrivers`

```go
// PickRandomDrivers randomly selects up to n driver IDs from pool using a Fisher-Yates shuffle.
// If len(pool) <= n, all drivers are returned in shuffled order.
func PickRandomDrivers(pool []types.ID, n int) []types.ID
```

**Properties:**
- O(len(pool)) time using an in-place Fisher-Yates shuffle on a copy.
- Does **not** mutate the input slice.
- Returns `nil` for `n ≤ 0` or empty pool.
- Each element appears at most once in the result (no duplicates).
- Over many runs, each driver is selected with uniform probability.

### Example

```go
// All 10 nearby drivers
pool := []types.ID{"d0", "d1", "d2", "d3", "d4", "d5", "d6", "d7", "d8", "d9"}

// Sample a pool of 10 then pick 5 to notify
pool10 := PickRandomDrivers(pool, selectPoolSize)   // up to 10
toNotify := PickRandomDrivers(pool10, notifyInitialCount) // 5 of those
```

## Scheduler Tick Logic

`RunScheduler` fires every `cfg.TickSeconds` seconds and calls `tickScheduledMatching`:

```
for each order in ListAvailableScheduled(now, now + 7 days):
    if order.dispatchedAt not found in Redis:
        dispatchInitial(order)           // T0
    else if order is NOT broadcast:
        if order.ScheduledAt - now <= 24h:
            broadcastWider(order)        // T-24h
            markBroadcast(order)
        else if now - dispatchedAt >= 30s:
            markBroadcast(order)         // T30s
```

### `dispatchInitial`

1. Query `NearbyDrivers(pickup, radiusKm)` from Redis GEO.
2. `PickRandomDrivers(nearby, 10)` → candidate pool.
3. `PickRandomDrivers(pool, 5)` → drivers to notify.
4. `RecordDispatch(orderID, toNotify)` stores dispatch timestamp in Redis.
5. `Notifier.NotifyDriver(...)` sends push notifications (if configured).

### `broadcastWider`

1. Query `NearbyDrivers(pickup, radiusKm * 2)` (double radius).
2. `PickRandomDrivers(nearby, 10)` → additional drivers to notify.
3. Send notifications.

## Redis Key Schema

| Key pattern                           | Type   | TTL  | Description                         |
|---------------------------------------|--------|------|-------------------------------------|
| `matching:drivers`                    | GEO    | —    | Active driver positions (GEO set)   |
| `matching:order:{id}:dispatched_at`   | String | 7 d  | RFC3339 timestamp of first dispatch |
| `matching:order:{id}:notified`        | Set    | 7 d  | Driver IDs notified at T0           |
| `matching:order:{id}:broadcast`       | String | 7 d  | "1" when order opened to all drivers|

## Interfaces

### `MatchingStore`

```go
type MatchingStore interface {
    AddCandidate(ctx context.Context, c Candidate) error
    RemoveCandidate(ctx context.Context, id types.ID) error
    NearbyDrivers(ctx context.Context, p types.Point, radiusKm float64) ([]types.ID, error)
    RecordDispatch(ctx context.Context, orderID types.ID, driverIDs []types.ID) error
    GetDispatchedAt(ctx context.Context, orderID types.ID) (time.Time, bool, error)
    MarkOrderBroadcast(ctx context.Context, orderID types.ID) error
    IsOrderBroadcast(ctx context.Context, orderID types.ID) (bool, error)
}
```

### `OrderMatcher`

```go
type OrderMatcher interface {
    Match(ctx context.Context, cmd order.MatchCommand) error
    Get(ctx context.Context, id types.ID) (*order.Order, error)
    ListAvailableScheduled(ctx context.Context, from, to time.Time) ([]*order.Order, error)
}
```

### `Notifier` (optional)

```go
type Notifier interface {
    NotifyDriver(ctx context.Context, driverID types.ID, orderID types.ID) error
}
```

If no `Notifier` is configured (via `SetNotifier`), push notifications are silently skipped.
The order is still correctly dispatch-tracked in Redis; drivers discover it via polling
`GET /api/orders/scheduled/available`.

## Test Coverage

`internal/modules/matching/matching_test.go` covers:

| Test | Description |
|------|-------------|
| `TestPickRandomDrivers_NormalCase` | Picks exactly n drivers; all from pool; all unique |
| `TestPickRandomDrivers_FewerThanN` | Returns entire pool when pool < n |
| `TestPickRandomDrivers_ExactN` | Handles pool size == n correctly |
| `TestPickRandomDrivers_EmptyPool` | Returns nil for nil / empty pool |
| `TestPickRandomDrivers_ZeroN` | Returns nil for n = 0 |
| `TestPickRandomDrivers_NegativeN` | Returns nil for n < 0 |
| `TestPickRandomDrivers_DoesNotMutatePool` | Original slice unchanged after call |
| `TestPickRandomDrivers_Distribution` | Each driver selected with uniform probability (1000 runs) |
| `TestMatchingNormalFlow` | Order dispatched on first tick; once claimed, not broadcast |
| `TestMatchingBroadcastAfterDelay` | Unclaimed order broadcast after T30s |
| `TestMatchingOneDayBroadcast` | Wider broadcast triggered within 24 h of pickup |
| `TestMatchingConcurrentPickRandom` | Goroutine-safe; each result unique and from pool |
| `TestMatchingFourRejectsThenAccept` | 4 passes without claim; 5th claims; order removed from available list |

## TODO

- **Broadcast if not taken 1 day before pickup**: ✅ Implemented as the T-24h phase above.
- **FCM / APNs Notifier**: Provide a concrete `Notifier` implementation using a push service.
- **Instant-order matching**: `TryImmediateMatch` picks one nearby driver and calls `order.Match`.
- **Driver location updates**: `AddCandidate` / `RemoveCandidate` feed the Redis GEO set.
