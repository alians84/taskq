package main

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/alians84/taskq/internal/api"
)

func runAPI(ctx context.Context) error {
	cfg, log, pool, err := setup(ctx)
	if err != nil {
		return err
	}

	ln, err := listen(ctx, cfg.HTTPAddr)
	if err != nil {
		pool.Close()
		return err
	}

	var shuttingDown atomic.Bool
	srv := &http.Server{
		Handler: api.NewHandler(api.Deps{
			Log:            log,
			DB:             pool,
			ShuttingDown:   &shuttingDown,
			MetricsHandler: metricsHandler(newPromRegistry()),
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	err = serve(ctx, log, srv, ln, cfg.ShutdownTimeout, func() { shuttingDown.Store(true) })
	pool.Close()
	log.InfoContext(ctx, "shutdown complete")
	return err
}
