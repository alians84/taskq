// Package api implements the taskq HTTP API.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
)

// Pinger reports whether the database is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps are the collaborators the HTTP handler needs.
type Deps struct {
	Log            *slog.Logger
	DB             Pinger
	ShuttingDown   *atomic.Bool
	MetricsHandler http.Handler
}

// NewHandler builds the root HTTP handler with all routes and middleware.
func NewHandler(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz(d))
	mux.Handle("GET /metrics", d.MetricsHandler)
	return withMiddleware(d.Log, mux)
}

// withMiddleware wraps h in the standard middleware chain:
// request ID → access log → panic recovery.
func withMiddleware(log *slog.Logger, h http.Handler) http.Handler {
	return requestID(logging(log)(recoverer(log)(h)))
}
