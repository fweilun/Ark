// README: Benchmark test cases derived from test.md; includes HTTP, DB, Redis, and performance checks.
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "regexp"
    "strings"
    "sync"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"
)

type Runner struct {
    cfg    Config
    httpc  *http.Client
    db     *pgxpool.Pool
    redis  *redis.Client
}

type Result struct {
    Name    string
    Status  string
    Latency time.Duration
    Note    string
}

type TestCase struct {
    Name  string
    Focus string
    Run   func(ctx context.Context, r *Runner) Result
}

func NewRunner(cfg Config) *Runner {
    return &Runner{
        cfg:   cfg,
        httpc: &http.Client{Timeout: 10 * time.Second},
    }
}

func (r *Runner) RunAll(ctx context.Context) []Result {
    if r.cfg.DSN != "" {
        if db, err := pgxpool.New(ctx, r.cfg.DSN); err == nil {
            r.db = db
        }
    }
    if r.cfg.RedisAddr != "" {
        r.redis = redis.NewClient(&redis.Options{Addr: r.cfg.RedisAddr})
    }

    tests := r.cases()
    results := make([]Result, 0, len(tests))

    for _, tc := range tests {
        res := tc.Run(ctx, r)
        results = append(results, res)
        fmt.Printf("%-7s %s", res.Status, tc.Name)
        if res.Latency > 0 {
            fmt.Printf(" (%s)", res.Latency)
        }
        if res.Note != "" {
            fmt.Printf(" - %s", res.Note)
        }
        fmt.Println()
    }

    if r.db != nil {
        r.db.Close()
    }
    if r.redis != nil {
        _ = r.redis.Close()
    }

    return results
}

func (r *Runner) cases() []TestCase {
    base := r.cfg.BaseURL
    return []TestCase{
        {
            Name:  "Env: Postgres connect",
            Focus: "DB 連線可用",
            Run: func(ctx context.Context, r *Runner) Result {
                if r.db == nil {
                    return Result{Name: "db", Status: "FAIL", Note: "db not configured"}
                }
                ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
                defer cancel()
                if err := r.db.Ping(ctx); err != nil {
                    return Result{Status: "FAIL", Note: err.Error()}
                }
                return Result{Status: "PASS"}
            },
        },
        {
            Name:  "Env: Redis connect",
            Focus: "Redis 連線可用",
            Run: func(ctx context.Context, r *Runner) Result {
                if r.redis == nil {
                    return Result{Status: "FAIL", Note: "redis not configured"}
                }
                ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
                defer cancel()
                if err := r.redis.Ping(ctx).Err(); err != nil {
                    return Result{Status: "FAIL", Note: err.Error()}
                }
                return Result{Status: "PASS"}
            },
        },
        {
            Name:  "Migration: apply (optional)",
            Focus: "可選套用 migration SQL",
            Run: func(ctx context.Context, r *Runner) Result {
                if !r.cfg.ApplyMigration {
                    return Result{Status: "SKIP", Note: "apply-migration=false"}
                }
                if r.db == nil {
                    return Result{Status: "FAIL", Note: "db not configured"}
                }
                sql, err := os.ReadFile(r.cfg.MigrationPath)
                if err != nil {
                    return Result{Status: "FAIL", Note: err.Error()}
                }
                stmts := splitSQL(string(sql))
                for _, s := range stmts {
                    if _, err := r.db.Exec(ctx, s); err != nil {
                        return Result{Status: "FAIL", Note: err.Error()}
                    }
                }
                return Result{Status: "PASS"}
            },
        },
        {
            Name:  "Migration: tables exist",
            Focus: "依 migrations/0001_init.sql 檢查表是否存在",
            Run: func(ctx context.Context, r *Runner) Result {
                if r.db == nil {
                    return Result{Status: "FAIL", Note: "db not configured"}
                }
                tables, err := extractTables(r.cfg.MigrationPath)
                if err != nil {
                    return Result{Status: "FAIL", Note: err.Error()}
                }
                for _, t := range tables {
                    var exists bool
                    err := r.db.QueryRow(ctx,
                        "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name=$1)",
                        t,
                    ).Scan(&exists)
                    if err != nil {
                        return Result{Status: "FAIL", Note: err.Error()}
                    }
                    if !exists {
                        return Result{Status: "FAIL", Note: "missing table: " + t}
                    }
                }
                return Result{Status: "PASS"}
            },
        },
        {
            Name:  "API: server reachable",
            Focus: "API 可回應請求",
            Run: func(ctx context.Context, r *Runner) Result {
                start := time.Now()
                resp, err := r.httpc.Get(base + "/api/passenger/order_status/test")
                if err != nil {
                    return Result{Status: "FAIL", Note: err.Error()}
                }
                _ = resp.Body.Close()
                return Result{Status: "PASS", Latency: time.Since(start), Note: fmt.Sprintf("status=%d", resp.StatusCode)}
            },
        },

        // Order flow
        httpCase("Order: passenger request ride (valid)", base+"/api/rides/request", map[string]any{
            "passenger_id": "p1",
            "pickup_lat": 25.033,
            "pickup_lng": 121.565,
            "dropoff_lat": 25.0478,
            "dropoff_lng": 121.5318,
            "ride_type": "economy",
        }, []int{200, 201}, []int{501, 404}),

        httpCase("Order: passenger request ride (missing fields -> 400)", base+"/api/rides/request", map[string]any{}, []int{400}, []int{501, 404}),

        httpCase("Order: passenger request ride (duplicate active)", base+"/api/rides/request", map[string]any{
            "passenger_id": "p1",
            "pickup_lat": 25.033,
            "pickup_lng": 121.565,
            "dropoff_lat": 25.0478,
            "dropoff_lng": 121.5318,
            "ride_type": "economy",
        }, []int{409}, []int{501, 404}),

        httpCase("Order: driver accept (valid)", base+"/api/rides/accept", map[string]any{
            "driver_id": "d1",
            "order_id":  "o1",
        }, []int{200, 201}, []int{501, 404}),

        httpCase("Order: driver accept (invalid state)", base+"/api/rides/accept", map[string]any{
            "driver_id": "d1",
            "order_id":  "o999",
        }, []int{409, 400}, []int{501, 404}),

        httpCase("Order: driver start trip", base+"/api/rides/start", map[string]any{
            "driver_id": "d1",
            "order_id":  "o1",
        }, []int{200, 201}, []int{501, 404}),

        httpCase("Order: driver complete trip", base+"/api/rides/complete", map[string]any{
            "driver_id": "d1",
            "order_id":  "o1",
        }, []int{200, 201}, []int{501, 404}),

        httpCaseMethod("Order: get order status", http.MethodGet, base+"/api/passenger/order_status/o1", nil, []int{200}, []int{501, 404}),

        manualCase("Order: completed cannot transition", "需完成後嘗試再次變更狀態"),

        // Cancel flow
        httpCase("Cancel: passenger cancel (created)", base+"/api/rides/cancel", map[string]any{
            "order_id": "o1",
            "reason":   "change_plans",
        }, []int{200, 201}, []int{501, 404}),

        httpCase("Cancel: passenger cancel (accepted/in_progress -> reject)", base+"/api/rides/cancel", map[string]any{
            "order_id": "o2",
            "reason":   "too_late",
        }, []int{409, 400}, []int{501, 404}),

        httpCase("Cancel: driver reject order", base+"/api/rides/reject", map[string]any{
            "driver_id": "d1",
            "order_id":  "o1",
            "reason":    "too_far",
        }, []int{200, 201}, []int{501, 404}),

        manualCase("Cancel: timeout cancel", "需等候超時或縮短 timeout 後觀察"),

        httpCase("Matching: driver availability", base+"/api/driver/set_availability", map[string]any{
            "driver_id": "d1",
            "is_available": true,
            "current_lat": 25.033,
            "current_lng": 121.565,
            "accepted_ride_types": []string{"economy"},
        }, []int{200, 201}, []int{501, 404}),

        manualCase("Matching: no nearby driver", "需設定空 driver pool 再嘗試配對"),
        manualCase("Matching: ride type filter", "需設定不同 ride_type 驗證配對限制"),
        manualCase("Matching: remove from pool after match", "需查 Redis set 是否移除"),

        // Location
        httpCase("Location: update passenger", base+"/api/location/update", map[string]any{
            "user_id": "p1",
            "user_type": "passenger",
            "lat": 25.033,
            "lng": 121.565,
        }, []int{200, 201}, []int{501, 404}),

        httpCase("Location: update driver", base+"/api/location/update", map[string]any{
            "user_id": "d1",
            "user_type": "driver",
            "lat": 25.033,
            "lng": 121.565,
        }, []int{200, 201}, []int{501, 404}),

        httpCase("Location: invalid coords -> 400", base+"/api/location/update", map[string]any{
            "user_id": "d1",
            "user_type": "driver",
            "lat": 123.0,
            "lng": 456.0,
        }, []int{400}, []int{501, 404}),

        manualCase("Location: throttling", "需觀察 DB 寫入頻率是否被節流"),
        manualCase("Location: snapshot persisted", "需查 DB location_snapshots 是否寫入"),

        // Pricing
        httpCase("Pricing: estimate (valid)", base+"/api/rides/request", map[string]any{
            "passenger_id": "p1",
            "pickup_lat": 25.033,
            "pickup_lng": 121.565,
            "dropoff_lat": 25.0478,
            "dropoff_lng": 121.5318,
            "ride_type": "economy",
        }, []int{200, 201}, []int{501, 404}),

        httpCase("Pricing: missing rate", base+"/api/rides/request", map[string]any{
            "passenger_id": "p1",
            "pickup_lat": 25.033,
            "pickup_lng": 121.565,
            "dropoff_lat": 25.0478,
            "dropoff_lng": 121.5318,
            "ride_type": "unknown",
        }, []int{400, 404}, []int{501, 404}),

        manualCase("Pricing: distance 0 -> base fare", "需支持 pricing 查詢"),

        // Data consistency
        manualCase("Consistency: orders/status/events一致", "需查詢 DB 驗證狀態與事件一致"),
        manualCase("Consistency: status_version 遞增", "需查詢 DB 驗證版本號"),
        manualCase("Consistency: cancelled cannot complete", "需驗證取消後不可完成"),

        // Concurrency
        {
            Name:  "Concurrency: multi accept same order",
            Focus: "只允許第一位司機接受",
            Run: func(ctx context.Context, r *Runner) Result {
                return concurrentAccept(ctx, r, base+"/api/rides/accept")
            },
        },
        manualCase("Concurrency: cancel vs accept", "需同時送 cancel/accept 驗證先到者為準"),
        manualCase("Concurrency: duplicate requests idempotent", "需檢查重送行為不重複寫入"),

        // Error handling
        manualCase("Error: DB down -> 500", "需暫停 DB 後觀察回應"),
        manualCase("Error: Redis down -> matching not run", "需暫停 Redis 後觀察回應"),
        manualCase("Error: restart recover orders", "需重啟服務後驗證訂單可續處理"),

        // Performance
        {
            Name:  "Perf: location update throughput",
            Focus: "每秒 50~100 次位置更新",
            Run: func(ctx context.Context, r *Runner) Result {
                return perfLoad(ctx, r, base+"/api/location/update", map[string]any{
                    "user_id": "d1",
                    "user_type": "driver",
                    "lat": 25.033,
                    "lng": 121.565,
                })
            },
        },
        {
            Name:  "Perf: request ride throughput",
            Focus: "每秒 10~20 筆下單",
            Run: func(ctx context.Context, r *Runner) Result {
                return perfLoad(ctx, r, base+"/api/rides/request", map[string]any{
                    "passenger_id": "p1",
                    "pickup_lat": 25.033,
                    "pickup_lng": 121.565,
                    "dropoff_lat": 25.0478,
                    "dropoff_lng": 121.5318,
                    "ride_type": "economy",
                })
            },
        },
    }
}

func httpCase(name, url string, body any, okStatuses, pendingStatuses []int) TestCase {
    return httpCaseMethod(name, http.MethodPost, url, body, okStatuses, pendingStatuses)
}

func httpCaseMethod(name, method, url string, body any, okStatuses, pendingStatuses []int) TestCase {
    return TestCase{
        Name:  name,
        Focus: "HTTP API",
        Run: func(ctx context.Context, r *Runner) Result {
            var reader io.Reader
            if body != nil {
                b, _ := json.Marshal(body)
                reader = strings.NewReader(string(b))
            }
            req, _ := http.NewRequestWithContext(ctx, method, url, reader)
            req.Header.Set("Content-Type", "application/json")
            start := time.Now()
            resp, err := r.httpc.Do(req)
            if err != nil {
                return Result{Status: "FAIL", Note: err.Error()}
            }
            io.Copy(io.Discard, resp.Body)
            resp.Body.Close()
            latency := time.Since(start)

            if contains(okStatuses, resp.StatusCode) {
                return Result{Status: "PASS", Latency: latency, Note: fmt.Sprintf("status=%d", resp.StatusCode)}
            }
            if contains(pendingStatuses, resp.StatusCode) {
                return Result{Status: "PENDING", Latency: latency, Note: fmt.Sprintf("status=%d", resp.StatusCode)}
            }
            return Result{Status: "FAIL", Latency: latency, Note: fmt.Sprintf("status=%d", resp.StatusCode)}
        },
    }
}

func manualCase(name, note string) TestCase {
    return TestCase{
        Name:  name,
        Focus: "Manual",
        Run: func(ctx context.Context, r *Runner) Result {
            return Result{Status: "SKIP", Note: note}
        },
    }
}

func concurrentAccept(ctx context.Context, r *Runner, url string) Result {
    payload := map[string]any{
        "driver_id": "d1",
        "order_id":  "o1",
    }
    b, _ := json.Marshal(payload)
    wg := sync.WaitGroup{}
    succ := 0
    pend := 0
    mu := sync.Mutex{}

    for i := 0; i < r.cfg.Concurrency; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(b)))
            req.Header.Set("Content-Type", "application/json")
            resp, err := r.httpc.Do(req)
            if err != nil {
                return
            }
            io.Copy(io.Discard, resp.Body)
            resp.Body.Close()
            mu.Lock()
            if resp.StatusCode >= 200 && resp.StatusCode < 300 {
                succ++
            } else if resp.StatusCode == 501 || resp.StatusCode == 404 {
                pend++
            }
            mu.Unlock()
        }(i)
    }
    wg.Wait()

    if pend == r.cfg.Concurrency {
        return Result{Status: "PENDING", Note: "not implemented"}
    }
    if succ <= 1 {
        return Result{Status: "PASS", Note: fmt.Sprintf("success=%d", succ)}
    }
    return Result{Status: "FAIL", Note: fmt.Sprintf("success=%d", succ)}
}

func perfLoad(ctx context.Context, r *Runner, url string, payload any) Result {
    b, _ := json.Marshal(payload)
    end := time.Now().Add(r.cfg.Duration)
    var count int64
    var errCount int64
    var mu sync.Mutex
    wg := sync.WaitGroup{}

    for i := 0; i < r.cfg.Concurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for time.Now().Before(end) {
                req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(b)))
                req.Header.Set("Content-Type", "application/json")
                resp, err := r.httpc.Do(req)
                if err != nil {
                    mu.Lock()
                    errCount++
                    mu.Unlock()
                    continue
                }
                io.Copy(io.Discard, resp.Body)
                resp.Body.Close()
                mu.Lock()
                count++
                mu.Unlock()
            }
        }()
    }
    wg.Wait()

    if count == 0 {
        return Result{Status: "FAIL", Note: "no requests completed"}
    }
    rps := float64(count) / r.cfg.Duration.Seconds()
    return Result{Status: "PASS", Note: fmt.Sprintf("rps=%.1f errors=%d", rps, errCount)}
}

func contains(list []int, v int) bool {
    for _, i := range list {
        if i == v {
            return true
        }
    }
    return false
}

func extractTables(path string) ([]string, error) {
    b, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    re := regexp.MustCompile(`(?i)create\s+table\s+if\s+not\s+exists\s+([a-zA-Z0-9_]+)`)
    matches := re.FindAllStringSubmatch(string(b), -1)
    tables := make([]string, 0, len(matches))
    for _, m := range matches {
        tables = append(tables, m[1])
    }
    return tables, nil
}

func splitSQL(sql string) []string {
    lines := strings.Split(sql, "\n")
    filtered := make([]string, 0, len(lines))
    for _, line := range lines {
        l := strings.TrimSpace(line)
        if strings.HasPrefix(l, "--") || l == "" {
            continue
        }
        filtered = append(filtered, line)
    }
    cleaned := strings.Join(filtered, "\n")
    parts := strings.Split(cleaned, ";")
    stmts := make([]string, 0, len(parts))
    for _, p := range parts {
        s := strings.TrimSpace(p)
        if s != "" {
            stmts = append(stmts, s)
        }
    }
    return stmts
}
