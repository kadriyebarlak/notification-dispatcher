package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL        string
	Port               string
	WorkerCount        int
	MaxRetries         int
	DispatcherInterval time.Duration
}

func LoadConfig() Config {
	return Config{
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://notify:notify@localhost:5432/notification_dispatcher?sslmode=disable"),
		Port:               getEnv("PORT", "8080"),
		WorkerCount:        getEnvInt("WORKER_COUNT", 3),
		MaxRetries:         getEnvInt("MAX_RETRIES", 3),
		DispatcherInterval: getEnvDuration("DISPATCHER_INTERVAL", 30*time.Second),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
