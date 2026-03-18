package config

import (
	"testing"
	"time"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("API_KEY", "token")
	t.Setenv("API_BASE_URL", "")
	t.Setenv("POLL_INTERVAL", "")
	t.Setenv("TASK_TIMEOUT", "")
	t.Setenv("REQUEST_TIMEOUT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.APIBaseURL != "https://api.example.com" {
		t.Fatalf("APIBaseURL = %q", cfg.APIBaseURL)
	}

	if cfg.PollInterval != defaultPollInterval {
		t.Fatalf("PollInterval = %v", cfg.PollInterval)
	}

	if cfg.TaskTimeout != defaultTaskTimeout {
		t.Fatalf("TaskTimeout = %v", cfg.TaskTimeout)
	}

	if cfg.RequestTimeout != defaultRequestTimeout {
		t.Fatalf("RequestTimeout = %v", cfg.RequestTimeout)
	}
}

func TestLoadParsesDurations(t *testing.T) {
	t.Setenv("API_KEY", "token")
	t.Setenv("POLL_INTERVAL", "2m")
	t.Setenv("TASK_TIMEOUT", "45s")
	t.Setenv("REQUEST_TIMEOUT", "10s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.PollInterval != 2*time.Minute {
		t.Fatalf("PollInterval = %v", cfg.PollInterval)
	}

	if cfg.TaskTimeout != 45*time.Second {
		t.Fatalf("TaskTimeout = %v", cfg.TaskTimeout)
	}

	if cfg.RequestTimeout != 10*time.Second {
		t.Fatalf("RequestTimeout = %v", cfg.RequestTimeout)
	}
}

func TestLoadRequiresAPIKey(t *testing.T) {
	t.Setenv("API_KEY", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() expected error for missing API_KEY")
	}
}
