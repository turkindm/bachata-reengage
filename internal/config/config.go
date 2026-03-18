package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultAPIBaseURL     = "https://lk.bachata.tech/json/v1.0"
	defaultPollInterval   = time.Minute
	defaultTaskTimeout    = 30 * time.Second
	defaultRequestTimeout = 15 * time.Second
	defaultMetricsAddr    = ":8080"
	defaultLookbackWindow = 8 * 24 * time.Hour
	defaultPageLimit      = 200
)

type Config struct {
	APIBaseURL     string
	APIToken       string
	DatabaseURL    string
	PollInterval   time.Duration
	TaskTimeout    time.Duration
	RequestTimeout time.Duration
	MetricsAddr    string
	LookbackWindow time.Duration
	PageLimit      int
}

func Load() (Config, error) {
	cfg := Config{
		APIBaseURL:     getenv("API_BASE_URL", defaultAPIBaseURL),
		APIToken:       os.Getenv("API_TOKEN"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		PollInterval:   durationEnv("POLL_INTERVAL", defaultPollInterval),
		TaskTimeout:    durationEnv("TASK_TIMEOUT", defaultTaskTimeout),
		RequestTimeout: durationEnv("REQUEST_TIMEOUT", defaultRequestTimeout),
		MetricsAddr:    getenv("METRICS_ADDR", defaultMetricsAddr),
		LookbackWindow: durationEnv("LOOKBACK_WINDOW", defaultLookbackWindow),
		PageLimit:      intEnv("PAGE_LIMIT", defaultPageLimit),
	}

	switch {
	case cfg.APIToken == "":
		return Config{}, fmt.Errorf("API_TOKEN is required")
	case cfg.DatabaseURL == "":
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	case cfg.LookbackWindow <= 0:
		return Config{}, fmt.Errorf("LOOKBACK_WINDOW must be positive")
	case cfg.PageLimit <= 0:
		return Config{}, fmt.Errorf("PAGE_LIMIT must be positive")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func intEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
