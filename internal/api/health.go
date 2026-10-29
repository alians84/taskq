package api

import (
	"context"
	"net/http"
	"time"
)

const readyPingTimeout = 2 * time.Second

func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readyz(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d.ShuttingDown.Load() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "shutting down"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), readyPingTimeout)
		defer cancel()
		if err := d.DB.Ping(ctx); err != nil {
			d.Log.WarnContext(ctx, "readiness: database ping failed", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}
