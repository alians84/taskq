package main

import (
	"context"
	"net/http"
	"time"
)

// runWorker is a placeholder until Task 3.5 wires the real worker loop:
// it only exposes metrics and shuts down gracefully.
func runWorker(ctx context.Context) error {
	cfg, log, pool, err := setup(ctx)
	if err != nil {
		return err
	}

	ln, err := listen(ctx, cfg.MetricsAddr)
	if err != nil {
		pool.Close()
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", metricsHandler(newPromRegistry()))
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	log.InfoContext(ctx, "worker loop not implemented yet", "worker_id", cfg.WorkerID)
	err = serve(ctx, log, srv, ln, cfg.ShutdownTimeout, nil)
	pool.Close()
	log.InfoContext(ctx, "shutdown complete")
	return err
}
