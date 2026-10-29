// Package config loads process configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

// Config holds settings for every taskq subcommand.
type Config struct {
	DatabaseURL       string
	LogLevel          slog.Level
	HTTPAddr          string
	MetricsAddr       string
	WorkerID          string
	WorkerConcurrency int
	PollInterval      time.Duration
	JobTimeout        time.Duration
	LeaseMargin       time.Duration
	ShutdownTimeout   time.Duration
}

// Load reads the configuration using getenv (os.Getenv in production).
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		DatabaseURL:       getenv("DATABASE_URL"),
		LogLevel:          slog.LevelInfo,
		HTTPAddr:          stringOr(getenv, "HTTP_ADDR", ":8080"),
		MetricsAddr:       stringOr(getenv, "METRICS_ADDR", ":9090"),
		WorkerID:          stringOr(getenv, "WORKER_ID", defaultWorkerID()),
		WorkerConcurrency: 10,
		PollInterval:      time.Second,
		JobTimeout:        30 * time.Second,
		LeaseMargin:       10 * time.Second,
		ShutdownTimeout:   25 * time.Second,
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("config: DATABASE_URL is required")
	}

	var err error
	if cfg.WorkerConcurrency, err = positiveInt(getenv, "WORKER_CONCURRENCY", cfg.WorkerConcurrency); err != nil {
		return Config{}, err
	}
	if cfg.PollInterval, err = duration(getenv, "POLL_INTERVAL", cfg.PollInterval, false); err != nil {
		return Config{}, err
	}
	if cfg.JobTimeout, err = duration(getenv, "JOB_TIMEOUT", cfg.JobTimeout, false); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = duration(getenv, "SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout, false); err != nil {
		return Config{}, err
	}
	if cfg.LeaseMargin, err = duration(getenv, "LEASE_MARGIN", cfg.LeaseMargin, true); err != nil {
		return Config{}, err
	}
	if v := getenv("LOG_LEVEL"); v != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(v)); err != nil {
			return Config{}, fmt.Errorf("config: LOG_LEVEL must be one of debug, info, warn, error, got %q", v)
		}
	}
	return cfg, nil
}

func stringOr(getenv func(string) string, key, def string) string {
	if v := getenv(key); v != "" {
		return v
	}
	return def
}

func positiveInt(getenv func(string) string, key string, def int) (int, error) {
	v := getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("config: %s must be a positive integer, got %q", key, v)
	}
	return n, nil
}

func defaultWorkerID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "worker"
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}

// duration parses key as a time.Duration. Zero is accepted only when allowZero is set.
func duration(getenv func(string) string, key string, def time.Duration, allowZero bool) (time.Duration, error) {
	v := getenv(key)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 || (d == 0 && !allowZero) {
		want := "a positive"
		if allowZero {
			want = "a non-negative"
		}
		return 0, fmt.Errorf("config: %s must be %s duration, got %q", key, want, v)
	}
	return d, nil
}
