package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
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

// listen binds addr before serving, so a busy port fails fast at startup.
func listen(ctx context.Context, addr string) (net.Listener, error) {
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	return ln, nil
}

// serve runs srv on ln until ctx is cancelled, then calls beforeShutdown (if set)
// and shuts the server down gracefully within timeout.
func serve(ctx context.Context, log *slog.Logger, srv *http.Server, ln net.Listener, timeout time.Duration, beforeShutdown func()) error {
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	log.InfoContext(ctx, "http server started", "addr", ln.Addr().String())

	select {
	case err := <-errc:
		return fmt.Errorf("http server %s: %w", ln.Addr(), err)
	case <-ctx.Done():
	}

	log.InfoContext(ctx, "shutdown started", "timeout", timeout.String())
	if beforeShutdown != nil {
		beforeShutdown()
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Requests still running past the deadline are cut off: closing their
		// connections cancels their contexts, so they release DB connections
		// and pool.Close does not block until the container is killed.
		_ = srv.Close()
		return fmt.Errorf("http server %s shutdown: %w", ln.Addr(), err)
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server %s: %w", ln.Addr(), err)
	}
	return nil
}
