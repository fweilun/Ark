# 預約訂單實作指南（MVP）

以下內容是直接可落地的實作 guide，包含資料庫 SQL、路由、Go function 介面與 JWT 驗證導入方式。

## 1. 資料庫與現有 schema 整合
基於 `migrations/0001_init.sql`，新增一個 migration 檔案，避免破壞既有資料。

新增檔案：`migrations/0002_schedule.sql`

```sql
-- migrations/0002_schedule.sql

-- 使用 Firebase UID 當主鍵
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    role TEXT NOT NULL, -- passenger | driver | admin
    display_name TEXT,
    email TEXT,
    phone TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- 擴充 orders 支援預約
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS order_type TEXT NOT NULL DEFAULT 'instant',
    ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS schedule_window_mins INT,
    ADD COLUMN IF NOT EXISTS cancel_deadline_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS incentive_bonus BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMP;

-- 索引：司機查詢可領取預約、乘客/司機查詢自己的訂單
CREATE INDEX IF NOT EXISTS idx_orders_scheduled_open
    ON orders (status, scheduled_at);
CREATE INDEX IF NOT EXISTS idx_orders_passenger_time
    ON orders (passenger_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_driver_time
    ON orders (driver_id, created_at DESC);
```

整合要點：
1. `orders.passenger_id` 與 `orders.driver_id` 直接對應 `users.id`（Firebase UID）。
2. 預約單用 `order_type = 'scheduled'`，即時單保留 `order_type = 'instant'`。
3. `status` 擴充為 `scheduled` 與 `assigned`，其餘狀態沿用。

## 2. 狀態與模型（Go）
新增狀態與欄位（示意放在 `internal/modules/order/model.go`）。

```go
const (
    StatusScheduled Status = "scheduled" // 可被司機領取
    StatusAssigned  Status = "assigned"  // 司機已領取，未到出發時間
)

// AllowedTransitions 要新增：
// scheduled -> assigned / cancelled
// assigned  -> approaching / cancelled / scheduled (司機取消後重新開放)

type Order struct {
    // ...existing fields...
    OrderType          string
    ScheduledAt        *time.Time
    ScheduleWindowMins *int
    CancelDeadlineAt   *time.Time
    IncentiveBonus     int64
    AssignedAt         *time.Time
}
```

## 3. Routes（Gin）
新增預約單的 API（沿用 orders 命名風格）。

```go
// internal/http/router.go
r.POST("/api/orders/scheduled", orderHandler.CreateScheduled)
r.GET("/api/orders/scheduled", orderHandler.ListScheduledByPassenger)
r.GET("/api/orders/scheduled/available", orderHandler.ListAvailableScheduled)
r.POST("/api/orders/:id/claim", orderHandler.Claim)
r.POST("/api/orders/:id/driver-cancel", orderHandler.DriverCancel)
// 乘客取消仍用既有: POST /api/orders/:id/cancel
```

建議 request/response（精簡）：
1. `POST /api/orders/scheduled`
   - body: `passenger_id, pickup_lat, pickup_lng, dropoff_lat, dropoff_lng, ride_type, scheduled_at (RFC3339), schedule_window_mins`
   - resp: `order_id, status=scheduled`
   - 驗證：`scheduled_at` 必須至少 30 分鐘後；`schedule_window_mins` 必須為正數；乘客不能有其他 active 訂單。
2. `GET /api/orders/scheduled?passenger_id=...`
   - resp: `{orders: [{order_id, status, scheduled_at, driver_id?, incentive_bonus, ...}]}`
3. `GET /api/orders/scheduled/available?from=RFC3339&to=RFC3339`
   - resp: `{orders: [{order_id, scheduled_at, pickup, ride_type, incentive_bonus}]}`
4. `POST /api/orders/:id/claim`
   - body: `{"driver_id": "..."}`
   - resp: `status=assigned`
5. `POST /api/orders/:id/driver-cancel`
   - body: `{"driver_id": "...", "reason": "..."}`
   - resp: `status=scheduled` (order re-opened with higher incentive_bonus)

## 4. Service functions（名稱 + 內容概況）
可放在 `internal/modules/order/schedule.go`

```go
// 建立預約單：驗證 scheduled_at、計算 cancel_deadline_at、寫入 orders + event
func (s *Service) CreateScheduled(ctx context.Context, cmd CreateScheduledCommand) (types.ID, error)

// 乘客查詢自己的預約單（含狀態、司機資訊）
func (s *Service) ListScheduledByPassenger(ctx context.Context, passengerID types.ID) ([]*Order, error)

// 司機查詢可領取預約單（依時間區間篩選）
func (s *Service) ListAvailableScheduled(ctx context.Context, from, to time.Time) ([]*Order, error)

// 司機領取（交易鎖定避免多人搶同一單）
func (s *Service) ClaimScheduled(ctx context.Context, cmd ClaimScheduledCommand) error

// 乘客取消（依 cancel_deadline_at 判斷是否可免費）
func (s *Service) CancelScheduledByPassenger(ctx context.Context, cmd CancelScheduledCommand) error

// 司機取消（重開放 + 提升獎勵）
func (s *Service) CancelScheduledByDriver(ctx context.Context, cmd DriverCancelScheduledCommand) error

// 排程任務：接近時間提高 incentive_bonus
func (s *Service) RunScheduleIncentiveTicker(ctx context.Context)

// 排程任務：超過截止時間仍無司機，更新狀態並通知
func (s *Service) RunScheduleExpireTicker(ctx context.Context)
```

## 5. Store functions（DB 操作 + SQL 概況）
放在 `internal/modules/order/store.go`。

```go
// 建立預約單
func (s *Store) CreateScheduled(ctx context.Context, o *Order) error
// SQL: INSERT INTO orders (..., order_type, scheduled_at, cancel_deadline_at, incentive_bonus, status)
// VALUES (..., 'scheduled', $scheduled_at, $cancel_deadline_at, $bonus, 'scheduled')

// 司機查詢可領取預約單
func (s *Store) ListAvailableScheduled(ctx context.Context, from, to time.Time) ([]*Order, error)
// SQL: SELECT ... FROM orders
// WHERE status = 'scheduled' AND scheduled_at BETWEEN $from AND $to

// 司機領取（樂觀鎖 or SELECT FOR UPDATE）
func (s *Store) ClaimScheduled(ctx context.Context, orderID, driverID types.ID, expectVersion int) (bool, error)
// SQL: UPDATE orders
// SET status='assigned', driver_id=$driver, assigned_at=NOW(), status_version=status_version+1
// WHERE id=$order AND status='scheduled' AND status_version=$expectVersion

// 乘客取消
func (s *Store) CancelScheduledByPassenger(ctx context.Context, orderID types.ID) error

// 司機取消後重新開放
func (s *Store) ReopenScheduled(ctx context.Context, orderID types.ID, bonus int64) error
// SQL: UPDATE orders
// SET status='scheduled', driver_id=NULL, assigned_at=NULL, incentive_bonus=incentive_bonus+$bonus
// WHERE id=$order AND status='assigned'
```

## 6. JWT 驗證導入（Firebase ID Token）
替換 `internal/http/middleware/auth.go` 內容，將 UID 放入 context。

```go
type AuthDeps struct {
    Firebase *auth.Client
    Users    *user.Store
}

func Auth(deps AuthDeps) gin.HandlerFunc {
    return func(c *gin.Context) {
        token := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
        if token == "" {
            c.AbortWithStatusJSON(401, gin.H{"error": "missing token"})
            return
        }
        decoded, err := deps.Firebase.VerifyIDToken(c.Request.Context(), token)
        if err != nil {
            c.AbortWithStatusJSON(401, gin.H{"error": "invalid token"})
            return
        }
        uid := decoded.UID
        _ = deps.Users.Upsert(c.Request.Context(), uid, decoded.Claims)
        c.Set("uid", uid)
        c.Next()
    }
}
```

Router 使用方式：

```go
r := gin.Default()
r.Use(middleware.Auth(middleware.AuthDeps{
    Firebase: firebaseClient,
    Users:    userStore,
}))
```
