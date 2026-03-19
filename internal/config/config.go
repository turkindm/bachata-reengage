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
	defaultLookbackWindow = 8 * 24 * time.Hour // 8 days (API max is 14 days)
	defaultMetricsAddr    = ":8080"
)

type Config struct {
	APIBaseURL     string
	APIToken       string
	OperatorLogin  string
	DatabaseURL    string
	PollInterval   time.Duration
	TaskTimeout    time.Duration
	RequestTimeout time.Duration
	LookbackWindow time.Duration
	MetricsAddr    string
	DryRun         bool
	TestDialogID   int64 // if non-zero: process only this dialog, run once and exit
}

func Load() (Config, error) {
	cfg := Config{
		APIBaseURL:     getenv("API_BASE_URL", defaultAPIBaseURL),
		APIToken:       os.Getenv("API_TOKEN"),
		OperatorLogin:  os.Getenv("OPERATOR_LOGIN"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		PollInterval:   durationEnv("POLL_INTERVAL", defaultPollInterval),
		TaskTimeout:    durationEnv("TASK_TIMEOUT", defaultTaskTimeout),
		RequestTimeout: durationEnv("REQUEST_TIMEOUT", defaultRequestTimeout),
		LookbackWindow: durationEnv("LOOKBACK_WINDOW", defaultLookbackWindow),
		MetricsAddr:    getenv("METRICS_ADDR", defaultMetricsAddr),
		DryRun:         os.Getenv("DRY_RUN") == "true",
		TestDialogID:   int64Env("TEST_DIALOG_ID"),
	}

	if cfg.APIToken == "" {
		return Config{}, fmt.Errorf("API_TOKEN is required")
	}

	if cfg.OperatorLogin == "" {
		return Config{}, fmt.Errorf("OPERATOR_LOGIN is required")
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.LookbackWindow <= 0 {
		return Config{}, fmt.Errorf("LOOKBACK_WINDOW must be positive")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func int64Env(key string) int64 {
	v, _ := strconv.ParseInt(os.Getenv(key), 10, 64)
	return v
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
