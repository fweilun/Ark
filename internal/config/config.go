// README: Config loader with env defaults for HTTP, DB, Redis, and matching settings.
package config

import (
	"os"
	"strconv"
)

type MatchingConfig struct {
	TickSeconds int
	RadiusKm    float64
}

type Config struct {
	HTTP struct {
		Addr string
	}
	DB struct {
		DSN string
	}
	Redis struct {
		Addr string
	}
	Matching MatchingConfig
	AI       struct {
		GeminiKey string
	}
}

func Load() (Config, error) {
	var cfg Config
	cfg.HTTP.Addr = envOrDefault("ARK_HTTP_ADDR", ":8080")
	cfg.DB.DSN = envOrDefault("ARK_DB_DSN", "postgres://postgres:postgres@localhost:5432/ark?sslmode=disable")
	cfg.Redis.Addr = envOrDefault("ARK_REDIS_ADDR", "localhost:6379")
	cfg.Matching.TickSeconds = envOrDefaultInt("ARK_MATCH_TICK", 3)
	cfg.Matching.RadiusKm = envOrDefaultFloat("ARK_MATCH_RADIUS_KM", 3.0)
	cfg.AI.GeminiKey = envOrError("GEMINI_API_KEY")
	return cfg, nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrError(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	panic("environment variable " + key + " is required")
}

func envOrDefaultInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envOrDefaultFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return def
}
