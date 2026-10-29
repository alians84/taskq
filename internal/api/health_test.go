package api

import (
	"errors"
	"net/http"
	"testing"
)

func TestHealthz(t *testing.T) {
	rec := do(t, NewHandler(testDeps()), http.MethodGet, "/healthz", nil)
	assertJSON(t, rec, http.StatusOK, `{"status":"ok"}`)
}

func TestReadyz_DatabaseUp(t *testing.T) {
	rec := do(t, NewHandler(testDeps()), http.MethodGet, "/readyz", nil)
	assertJSON(t, rec, http.StatusOK, `{"status":"ready"}`)
}

func TestReadyz_DatabaseDown(t *testing.T) {
	d := testDeps()
	d.DB = fakePinger{err: errors.New("connection refused")}
	rec := do(t, NewHandler(d), http.MethodGet, "/readyz", nil)
	assertJSON(t, rec, http.StatusServiceUnavailable, `{"error":"database unavailable"}`)
}

func TestReadyz_ShuttingDown(t *testing.T) {
	d := testDeps()
	d.ShuttingDown.Store(true)
	rec := do(t, NewHandler(d), http.MethodGet, "/readyz", nil)
	assertJSON(t, rec, http.StatusServiceUnavailable, `{"error":"shutting down"}`)
}

func TestMetrics_DelegatesToMetricsHandler(t *testing.T) {
	d := testDeps()
	d.MetricsHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("taskq_up 1\n"))
	})
	rec := do(t, NewHandler(d), http.MethodGet, "/metrics", nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "taskq_up 1\n" {
		t.Errorf("GET /metrics = %d %q, want 200 %q", rec.Code, rec.Body.String(), "taskq_up 1\n")
	}
}
