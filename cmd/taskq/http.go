package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// newPromRegistry returns a registry with the standard Go runtime and process collectors.
func newPromRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return reg
}

func metricsHandler(reg *prometheus.Registry) http.Handler {
	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg})
}

// serve runs srv until ctx is cancelled, then calls beforeShutdown (if set) and
// shuts the server down gracefully within timeout.
func serve(ctx context.Context, log *slog.Logger, srv *http.Server, timeout time.Duration, beforeShutdown func()) error {
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.InfoContext(ctx, "http server started", "addr", srv.Addr)

	select {
	case err := <-errc:
		return fmt.Errorf("http server %s: %w", srv.Addr, err)
	case <-ctx.Done():
	}

	log.InfoContext(ctx, "shutdown started", "timeout", timeout.String())
	if beforeShutdown != nil {
		beforeShutdown()
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http server %s shutdown: %w", srv.Addr, err)
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server %s: %w", srv.Addr, err)
	}
	return nil
}
