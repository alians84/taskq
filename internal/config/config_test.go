package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load(env(map[string]string{"DATABASE_URL": "postgres://localhost/taskq"}))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}

	if cfg.DatabaseURL != "postgres://localhost/taskq" {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://localhost/taskq")
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8080")
	}
	if cfg.MetricsAddr != ":9090" {
		t.Errorf("MetricsAddr = %q, want %q", cfg.MetricsAddr, ":9090")
	}
	if cfg.WorkerConcurrency != 10 {
		t.Errorf("WorkerConcurrency = %d, want 10", cfg.WorkerConcurrency)
	}
	if cfg.PollInterval != time.Second {
		t.Errorf("PollInterval = %v, want 1s", cfg.PollInterval)
	}
	if cfg.JobTimeout != 30*time.Second {
		t.Errorf("JobTimeout = %v, want 30s", cfg.JobTimeout)
	}
	if cfg.LeaseMargin != 10*time.Second {
		t.Errorf("LeaseMargin = %v, want 10s", cfg.LeaseMargin)
	}
	if cfg.ShutdownTimeout != 25*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 25s", cfg.ShutdownTimeout)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelInfo)
	}
	if cfg.WorkerID == "" || !strings.Contains(cfg.WorkerID, "-") {
		t.Errorf("WorkerID = %q, want non-empty <hostname>-<pid>", cfg.WorkerID)
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	_, err := Load(env(map[string]string{}))
	if err == nil {
		t.Fatal("Load: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("error = %q, want it to mention DATABASE_URL", err)
	}
}

func TestLoad_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "concurrency zero", key: "WORKER_CONCURRENCY", value: "0"},
		{name: "concurrency not a number", key: "WORKER_CONCURRENCY", value: "abc"},
		{name: "poll interval negative", key: "POLL_INTERVAL", value: "-1s"},
		{name: "poll interval garbage", key: "POLL_INTERVAL", value: "x"},
		{name: "job timeout zero", key: "JOB_TIMEOUT", value: "0s"},
		{name: "shutdown timeout zero", key: "SHUTDOWN_TIMEOUT", value: "0s"},
		{name: "lease margin negative", key: "LEASE_MARGIN", value: "-1s"},
		{name: "unknown log level", key: "LOG_LEVEL", value: "loud"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars := map[string]string{"DATABASE_URL": "postgres://localhost/taskq", tt.key: tt.value}
			_, err := Load(env(vars))
			if err == nil {
				t.Fatalf("Load with %s=%q: expected error, got nil", tt.key, tt.value)
			}
			if !strings.Contains(err.Error(), tt.key) {
				t.Errorf("error = %q, want it to mention %s", err, tt.key)
			}
		})
	}
}

func TestLoad_Overrides(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"DATABASE_URL":       "postgres://db/taskq",
		"LOG_LEVEL":          "debug",
		"HTTP_ADDR":          ":18080",
		"METRICS_ADDR":       ":19090",
		"WORKER_ID":          "worker-a",
		"WORKER_CONCURRENCY": "3",
		"POLL_INTERVAL":      "250ms",
		"JOB_TIMEOUT":        "5s",
		"LEASE_MARGIN":       "0s",
		"SHUTDOWN_TIMEOUT":   "7s",
	}))
	if err != nil {
		t.Fatalf("Load: unexpected error: %v", err)
	}

	want := Config{
		DatabaseURL:       "postgres://db/taskq",
		LogLevel:          slog.LevelDebug,
		HTTPAddr:          ":18080",
		MetricsAddr:       ":19090",
		WorkerID:          "worker-a",
		WorkerConcurrency: 3,
		PollInterval:      250 * time.Millisecond,
		JobTimeout:        5 * time.Second,
		LeaseMargin:       0,
		ShutdownTimeout:   7 * time.Second,
	}
	if cfg != want {
		t.Errorf("Load:\n got  %+v\n want %+v", cfg, want)
	}
}
