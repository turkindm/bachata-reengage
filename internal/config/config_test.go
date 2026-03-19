package config

import (
	"testing"
	"time"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("API_TOKEN", "token")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("API_BASE_URL", "")
	t.Setenv("POLL_INTERVAL", "")
	t.Setenv("TASK_TIMEOUT", "")
	t.Setenv("REQUEST_TIMEOUT", "")
	t.Setenv("LOOKBACK_WINDOW", "")
	t.Setenv("METRICS_ADDR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.APIBaseURL != defaultAPIBaseURL {
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
	if cfg.LookbackWindow != defaultLookbackWindow {
		t.Fatalf("LookbackWindow = %v", cfg.LookbackWindow)
	}
	if cfg.MetricsAddr != defaultMetricsAddr {
		t.Fatalf("MetricsAddr = %q", cfg.MetricsAddr)
	}
}

func TestLoadParsesDurations(t *testing.T) {
	t.Setenv("API_TOKEN", "token")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("POLL_INTERVAL", "2m")
	t.Setenv("TASK_TIMEOUT", "45s")
	t.Setenv("REQUEST_TIMEOUT", "10s")
	t.Setenv("LOOKBACK_WINDOW", "168h")

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
	if cfg.LookbackWindow != 168*time.Hour {
		t.Fatalf("LookbackWindow = %v", cfg.LookbackWindow)
	}
}

func TestLoadRequiresAPIToken(t *testing.T) {
	t.Setenv("API_TOKEN", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")

	if _, err := Load(); err == nil {
		t.Fatal("Load() expected error for missing API_TOKEN")
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("API_TOKEN", "token")
	t.Setenv("DATABASE_URL", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() expected error for missing DATABASE_URL")
	}
}
