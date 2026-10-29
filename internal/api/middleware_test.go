package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

var hex32 = regexp.MustCompile(`^[0-9a-f]{32}$`)

func TestRequestID_GeneratedWhenMissing(t *testing.T) {
	rec := do(t, NewHandler(testDeps()), http.MethodGet, "/healthz", nil)
	if id := rec.Header().Get("X-Request-Id"); !hex32.MatchString(id) {
		t.Errorf("X-Request-Id = %q, want 32 lowercase hex characters", id)
	}
}

func TestRequestID_EchoesValidIncoming(t *testing.T) {
	h := http.Header{"X-Request-Id": {"abc-123"}}
	rec := do(t, NewHandler(testDeps()), http.MethodGet, "/healthz", h)
	if id := rec.Header().Get("X-Request-Id"); id != "abc-123" {
		t.Errorf("X-Request-Id = %q, want %q", id, "abc-123")
	}
}

func TestRequestID_ReplacesInvalidIncoming(t *testing.T) {
	tests := []struct {
		name     string
		incoming string
	}{
		{name: "longer than 64 characters", incoming: strings.Repeat("a", 65)},
		{name: "characters outside [A-Za-z0-9_-]", incoming: "id\nforged=1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.Header{"X-Request-Id": {tt.incoming}}
			rec := do(t, NewHandler(testDeps()), http.MethodGet, "/healthz", h)
			if id := rec.Header().Get("X-Request-Id"); !hex32.MatchString(id) {
				t.Errorf("X-Request-Id = %q, want a freshly generated 32-hex ID", id)
			}
		})
	}
}

func TestLogging_RecordsRequest(t *testing.T) {
	var buf bytes.Buffer
	d := testDeps()
	d.Log = slog.New(slog.NewJSONHandler(&buf, nil))
	d.ShuttingDown.Store(true)

	do(t, NewHandler(d), http.MethodGet, "/readyz", http.Header{"X-Request-Id": {"req-1"}})

	var entry map[string]any
	for line := range strings.Lines(buf.String()) {
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err == nil && e["msg"] == "http request" {
			entry = e
		}
	}
	if entry == nil {
		t.Fatalf("no \"http request\" log entry in:\n%s", buf.String())
	}
	want := map[string]any{"method": "GET", "path": "/readyz", "status": float64(503), "request_id": "req-1"}
	for k, v := range want {
		if entry[k] != v {
			t.Errorf("log %s = %v, want %v", k, entry[k], v)
		}
	}
	if _, ok := entry["duration_ms"].(float64); !ok {
		t.Errorf("log duration_ms = %v, want a number", entry["duration_ms"])
	}
}

func TestRecover_PanicReturns500AndServerKeepsWorking(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /boom", func(http.ResponseWriter, *http.Request) { panic("boom") })
	mux.HandleFunc("GET /ok", healthz)
	h := withMiddleware(slog.New(slog.DiscardHandler), mux)

	rec := do(t, h, http.MethodGet, "/boom", nil)
	assertJSON(t, rec, http.StatusInternalServerError, `{"error":"internal error"}`)

	rec = do(t, h, http.MethodGet, "/ok", nil)
	assertJSON(t, rec, http.StatusOK, `{"status":"ok"}`)
}

func TestRecover_PanicAfterHeadersAbortsResponse(t *testing.T) {
	var logs bytes.Buffer
	mux := http.NewServeMux()
	mux.HandleFunc("GET /half", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"partial":`))
		panic("boom")
	})
	h := withMiddleware(slog.New(slog.NewJSONHandler(&logs, nil)), mux)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/half", nil)
	func() {
		defer func() {
			if v := recover(); v != http.ErrAbortHandler { //nolint:errorlint // sentinel compared by identity, as net/http does
				t.Errorf("panic value = %v, want http.ErrAbortHandler so net/http drops the connection", v)
			}
		}()
		h.ServeHTTP(rec, req)
	}()

	if strings.Contains(rec.Body.String(), "internal error") {
		t.Errorf("body = %q: an error body must not be appended to an already started response", rec.Body.String())
	}
	if !strings.Contains(logs.String(), "http handler panic") {
		t.Errorf("logs do not contain the original panic:\n%s", logs.String())
	}
}
