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
	t.Setenv("PAGE_LIMIT", "")
	t.Setenv("METRICS_ADDR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.APIBaseURL != defaultAPIBaseURL || cfg.MetricsAddr != defaultMetricsAddr || cfg.PageLimit != defaultPageLimit {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestLoadParsesValues(t *testing.T) {
	t.Setenv("API_TOKEN", "token")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("POLL_INTERVAL", "2m")
	t.Setenv("TASK_TIMEOUT", "45s")
	t.Setenv("REQUEST_TIMEOUT", "10s")
	t.Setenv("LOOKBACK_WINDOW", "192h")
	t.Setenv("PAGE_LIMIT", "50")
	t.Setenv("METRICS_ADDR", ":9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.PollInterval != 2*time.Minute || cfg.TaskTimeout != 45*time.Second || cfg.RequestTimeout != 10*time.Second || cfg.LookbackWindow != 192*time.Hour || cfg.PageLimit != 50 || cfg.MetricsAddr != ":9090" {
		t.Fatalf("cfg = %#v", cfg)
	}
}

func TestLoadRequiresSecrets(t *testing.T) {
	t.Setenv("API_TOKEN", "")
	t.Setenv("DATABASE_URL", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() expected error")
	}
}
