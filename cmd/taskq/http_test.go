package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

// A request that outlives the shutdown timeout must be cut off, otherwise the
// process keeps holding DB connections and pool.Close blocks until SIGKILL.
func TestServe_ForceClosesConnectionsAfterShutdownTimeout(t *testing.T) {
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	started := make(chan struct{})
	handlerDone := make(chan struct{})
	srv := &http.Server{Handler: http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(handlerDone)
	})}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	served := make(chan error, 1)
	go func() {
		served <- serve(ctx, slog.New(slog.DiscardHandler), srv, ln, 100*time.Millisecond, nil)
	}()

	go func() {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+ln.Addr().String()+"/slow", nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()
	<-started
	cancel()

	select {
	case err := <-served:
		if err == nil {
			t.Error("serve returned nil, want an error reporting the shutdown timeout")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return within 5s")
	}
	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("in-flight request still running after serve returned: connections were not force-closed")
	}
}
