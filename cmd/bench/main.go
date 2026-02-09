// README: Benchmark runner for MVP test cases; executes HTTP/DB/Redis checks and prints results.
package main

import (
    "context"
    "flag"
    "fmt"
    "os"
    "strings"
    "time"
)

func main() {
    cfg := loadConfig()

    ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
    defer cancel()

    bench := NewRunner(cfg)
    results := bench.RunAll(ctx)

    fmt.Println("\n== Summary ==")
    pass, fail, pending, skipped := 0, 0, 0, 0
    for _, r := range results {
        switch r.Status {
        case "PASS":
            pass++
        case "FAIL":
            fail++
        case "PENDING":
            pending++
        case "SKIP":
            skipped++
        }
    }
    fmt.Printf("PASS=%d FAIL=%d PENDING=%d SKIP=%d\n", pass, fail, pending, skipped)

    if cfg.Strict && (fail > 0 || pending > 0) {
        os.Exit(1)
    }
    if fail > 0 {
        os.Exit(1)
    }
}

type Config struct {
    BaseURL       string
    DSN           string
    RedisAddr     string
    MigrationPath string
    ApplyMigration bool
    Strict        bool
    Timeout       time.Duration
    Concurrency   int
    Duration      time.Duration
}

func loadConfig() Config {
    var cfg Config
    flag.StringVar(&cfg.BaseURL, "base-url", envOrDefault("ARK_BENCH_BASE_URL", "http://localhost:8080"), "API base URL")
    flag.StringVar(&cfg.DSN, "dsn", envOrDefault("ARK_DB_DSN", "postgres://postgres:postgres@localhost:5432/ark?sslmode=disable"), "Postgres DSN")
    flag.StringVar(&cfg.RedisAddr, "redis", envOrDefault("ARK_REDIS_ADDR", "localhost:6379"), "Redis address")
    flag.StringVar(&cfg.MigrationPath, "migration", envOrDefault("ARK_BENCH_MIGRATION", "migrations/0001_init.sql"), "Migration SQL path")
    flag.BoolVar(&cfg.ApplyMigration, "apply-migration", envOrDefaultBool("ARK_BENCH_APPLY_MIGRATION", false), "Apply migration SQL before tests")
    flag.BoolVar(&cfg.Strict, "strict", envOrDefaultBool("ARK_BENCH_STRICT", false), "Fail on pending tests")
    flag.DurationVar(&cfg.Timeout, "timeout", envOrDefaultDuration("ARK_BENCH_TIMEOUT", 60*time.Second), "Total timeout")
    flag.IntVar(&cfg.Concurrency, "concurrency", envOrDefaultInt("ARK_BENCH_CONCURRENCY", 20), "Concurrency for perf tests")
    flag.DurationVar(&cfg.Duration, "duration", envOrDefaultDuration("ARK_BENCH_DURATION", 10*time.Second), "Duration for perf tests")
    flag.Parse()
    cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
    return cfg
}

func envOrDefault(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}

func envOrDefaultBool(key string, def bool) bool {
    if v := os.Getenv(key); v != "" {
        v = strings.ToLower(v)
        return v == "1" || v == "true" || v == "yes"
    }
    return def
}

func envOrDefaultInt(key string, def int) int {
    if v := os.Getenv(key); v != "" {
        var n int
        _, _ = fmt.Sscanf(v, "%d", &n)
        if n > 0 {
            return n
        }
    }
    return def
}

func envOrDefaultDuration(key string, def time.Duration) time.Duration {
    if v := os.Getenv(key); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            return d
        }
    }
    return def
}
