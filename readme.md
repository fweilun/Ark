# Ark API

## Quick Start (Docker)

Run the following command to start the API, Postgres, and Redis:

```bash
docker-compose up --build
```

The API will be available at `http://localhost:8080`.

- **Postgres**: Exposed on port `5432`.
- **Redis**: Exposed on port `6379`.
- **Database Initialization**: The `migrations/0001_init.sql` script is automatically applied on the first run.

## Components


## 1. 訂單管理器 (Order Manager Goroutine)

需要獨立於匹配管理器的組件，負責：
- 訂單生命週期管理 (created → matched → accepted → in_progress → completed/cancelled)
- 訂單狀態持久化到資料庫
- 超時處理（司機未接受、乘客取消等）
- 費用計算邏輯

## 2. 位置更新處理器 (Location Update Handler)

```go
// 需要獨立的 Goroutine 處理高頻位置更新
type LocationUpdate struct {
    UserID    string
    UserType  string // "driver" | "passenger"
    Lat       float64
    Lng       float64
    Timestamp int64
}

// 使用緩存層減少 DB 壓力
locationCache := make(map[string]LocationUpdate)
```

## 3. 會話管理 (Session Manager)

```go
// 管理用戶連線狀態
type UserSession struct {
    UserID      string
    UserType    string
    Connection  *websocket.Conn  // WebSocket 連線
    LastSeen    time.Time
    IsActive    bool
}
```

# 🔄 API 介面詳細定義

## 乘客端 API (需要補充)

```json
// POST /api/passenger/request_ride
{
  "passenger_id": "uuid",
  "pickup_lat": 25.0330,
  "pickup_lng": 121.5654,
  "dropoff_lat": 25.0478,
  "dropoff_lng": 121.5318,
  "ride_type": "economy|premium|pool",
  "payment_method": "cash|card"
}

// POST /api/passenger/cancel_ride
{
  "order_id": "uuid",
  "reason": "waiting_too_long|change_plans"
}

// GET /api/passenger/order_status/{order_id}
```

## 司機端 API (需要補充)

```json
// POST /api/driver/set_availability
{
  "driver_id": "uuid",
  "is_available": true,
  "current_lat": 25.0330,
  "current_lng": 121.5654,
  "accepted_ride_types": ["economy", "premium"]
}

// POST /api/driver/accept_order
{
  "driver_id": "uuid",
  "order_id": "uuid",
  "estimated_arrival": 300  // 秒
}

// POST /api/driver/reject_order
{
  "driver_id": "uuid",
  "order_id": "uuid",
  "reason": "too_far|break_time"
}
```

# 🏗️ 資料庫設計補充

## 訂單表 (orders)

```sql
CREATE TABLE orders (
    id UUID PRIMARY KEY,
    passenger_id UUID REFERENCES passengers(id),
    driver_id UUID REFERENCES drivers(id) NULLABLE,
    status VARCHAR(50), -- 'pending', 'matched', 'accepted', 'in_progress', 'completed', 'cancelled'
    pickup_lat FLOAT,
    pickup_lng FLOAT,
    dropoff_lat FLOAT,
    dropoff_lng FLOAT,
    ride_type VARCHAR(20),
    estimated_fee DECIMAL(10,2),
    actual_fee DECIMAL(10,2) NULLABLE,
    created_at TIMESTAMP,
    accepted_at TIMESTAMP NULLABLE,
    started_at TIMESTAMP NULLABLE,
    completed_at TIMESTAMP NULLABLE,
    cancelled_at TIMESTAMP NULLABLE,
    cancellation_reason VARCHAR(100) NULLABLE
);
```

## 位置快照表 (location_snapshots)

```sql
CREATE TABLE location_snapshots (
    id SERIAL PRIMARY KEY,
    user_id UUID,
    user_type VARCHAR(10),
    lat FLOAT,
    lng FLOAT,
    recorded_at TIMESTAMP DEFAULT NOW(),
    INDEX idx_user_time (user_id, recorded_at DESC)
);
```

# 🎯 關鍵 Goroutine 設計

## 1. 匹配排程器 (Match Scheduler)

```go
func matchScheduler() {
    ticker := time.NewTicker(3 * time.Second) // 每3秒匹配一次
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            // 批次匹配邏輯
            candidates := getMatchingCandidates()
            matches := matchingAlgorithm(candidates)
            processMatches(matches)

        case newUser := <-newUserChan:
            // 新用戶立即嘗試匹配
            tryImmediateMatch(newUser)
        }
    }
}
```

## 2. 訂單狀態監控器

```go
func orderMonitor() {
    for {
        // 檢查超時訂單
        timeoutOrders := getTimeoutOrders()
        for _, order := range timeoutOrders {
            handleOrderTimeout(order)
        }

        // 檢查長時間等待的乘客
        longWaitPassengers := getLongWaitPassengers()
        for _, passenger := range longWaitPassengers {
            notifyLongWait(passenger)
        }

        time.Sleep(30 * time.Second)
    }
}
```

# ⚡ 效能優化建議

## 1. Redis 快取層

```go
// 儲存即時匹配資訊
redisClient := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

// Key 設計
// active_passengers:{zone_id}
// active_drivers:{zone_id}
// order:{order_id}:status
```

## 2. 地理分區 (Geo-partitioning)

```go
// 將地圖分為網格，只在同區或鄰區匹配
func getGridZone(lat, lng float64) string {
    gridSize := 0.01 // 約1公里
    gridX := int(lat / gridSize)
    gridY := int(lng / gridSize)
    return fmt.Sprintf("%d_%d", gridX, gridY)
}
```

## 3. 批次處理位置更新

```go
func locationBatchProcessor() {
    batch := make([]LocationUpdate, 0, 100)
    batchTimer := time.NewTicker(5 * time.Second)

    for {
        select {
        case loc := <-locationChan:
            batch = append(batch, loc)
            if len(batch) >= 100 {
                saveLocationBatch(batch)
                batch = batch[:0]
            }
        case <-batchTimer.C:
            if len(batch) > 0 {
                saveLocationBatch(batch)
                batch = batch[:0]
            }
        }
    }
}
```

# 🛡️ 錯誤處理與監控

需要添加的監控指標：

```go
type Metrics struct {
    ActivePassengers     int
    ActiveDrivers        int
    MatchSuccessRate     float64
    AvgMatchTime         float64
    AvgResponseTime      float64
    OrderCompletionRate  float64
    CancellationRate     float64
}
```

Circuit Breaker 模式：

```go
// 對於外部服務（地圖API、支付API等）
circuitBreaker := gobreaker.NewCircuitBreaker(
    gobreaker.Settings{
        Name:        "map-api",
        MaxRequests: 5,
        Interval:    10 * time.Second,
        Timeout:     5 * time.Second,
    },
)
```

# 📋 下一步行動清單

- 立即實作：訂單管理器和位置更新處理器
- API擴充：完善乘客/司機的訂單操作端點
- 資料庫：建立訂單相關表格
- 監控：添加基本的prometheus指標
- 測試：撰寫壓力測試，模擬萬人同時叫車
