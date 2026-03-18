package config

import (
	"fmt"
	"os"
	"time"
)

const (
	defaultPollInterval   = time.Minute
	defaultTaskTimeout    = 30 * time.Second
	defaultRequestTimeout = 15 * time.Second
)

type Config struct {
	APIBaseURL     string
	APIKey         string
	PollInterval   time.Duration
	TaskTimeout    time.Duration
	RequestTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		APIBaseURL:     getenv("API_BASE_URL", "https://api.example.com"),
		APIKey:         os.Getenv("API_KEY"),
		PollInterval:   durationEnv("POLL_INTERVAL", defaultPollInterval),
		TaskTimeout:    durationEnv("TASK_TIMEOUT", defaultTaskTimeout),
		RequestTimeout: durationEnv("REQUEST_TIMEOUT", defaultRequestTimeout),
	}

	if cfg.APIKey == "" {
		return Config{}, fmt.Errorf("API_KEY is required")
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
