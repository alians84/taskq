// Command taskq runs the taskq API, worker, or database migrations.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alians84/taskq/internal/config"
)

const usage = `usage: taskq <command>

commands:
  api          run the HTTP API
  worker       run the job worker
  migrate up   apply database migrations
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch {
	case len(args) == 1 && args[0] == "api":
		err = runAPI(ctx)
	case len(args) == 1 && args[0] == "worker":
		err = runWorker(ctx)
	case len(args) == 2 && args[0] == "migrate" && args[1] == "up":
		err = runMigrate(ctx)
	default:
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "taskq: %v\n", err)
		return 1
	}
	return 0
}

// setup loads configuration, builds the logger and connects to Postgres.
func setup(ctx context.Context) (config.Config, *slog.Logger, *pgxpool.Pool, error) {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return config.Config{}, nil, nil, err
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("connect to postgres: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return config.Config{}, nil, nil, fmt.Errorf("ping postgres: %w", err)
	}
	return cfg, log, pool, nil
}
