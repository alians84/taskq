//go:build integration

package testdb

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestPool_MigrationsApplied(t *testing.T) {
	ctx := context.Background()
	pool := Pool(t)

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM jobs").Scan(&count); err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	if count != 0 {
		t.Fatalf("jobs count = %d, want 0", count)
	}

	var (
		status      string
		attempts    int
		maxAttempts int
	)
	err := pool.QueryRow(ctx,
		`INSERT INTO jobs (type, payload) VALUES ('sleep', '{}')
		 RETURNING status, attempts, max_attempts`,
	).Scan(&status, &attempts, &maxAttempts)
	if err != nil {
		t.Fatalf("insert job with defaults: %v", err)
	}
	if status != "pending" || attempts != 0 || maxAttempts != 5 {
		t.Errorf("defaults = (%q, %d, %d), want (\"pending\", 0, 5)", status, attempts, maxAttempts)
	}

	_, err = pool.Exec(ctx, `INSERT INTO jobs (type, payload, status) VALUES ('sleep', '{}', 'bogus')`)
	if err == nil {
		t.Error("insert with status 'bogus': expected check violation, got nil")
	}

	pool = Pool(t)
	var id int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO jobs (type, payload) VALUES ('sleep', '{}') RETURNING id`,
	).Scan(&id); err != nil {
		t.Fatalf("insert after second Pool call: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM jobs").Scan(&count); err != nil {
		t.Fatalf("count jobs after second Pool call: %v", err)
	}
	if count != 1 || id != 1 {
		t.Errorf("after second Pool call: count = %d, id = %d, want count 1 and id 1", count, id)
	}
}

// fatalRecorder captures Fatalf instead of stopping the test, so we can assert
// that Pool fails (rather than hangs) when cleanup is blocked.
type fatalRecorder struct {
	testing.TB
	msg string
}

func (f *fatalRecorder) Helper() {}

func (f *fatalRecorder) Fatalf(format string, args ...any) { f.msg = fmt.Sprintf(format, args...) }

func TestPool_FailsFastWhenJobsTableIsLocked(t *testing.T) {
	ctx := context.Background()
	tx, err := Pool(t).Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// A leaked transaction holding any lock on jobs blocks TRUNCATE.
	if _, err := tx.Exec(ctx, "LOCK TABLE jobs IN ACCESS SHARE MODE"); err != nil {
		t.Fatalf("lock jobs: %v", err)
	}

	old := truncateTimeout
	truncateTimeout = 200 * time.Millisecond
	defer func() { truncateTimeout = old }()

	rec := &fatalRecorder{TB: t}
	start := time.Now()
	Pool(rec)
	if rec.msg == "" {
		t.Fatal("Pool succeeded while jobs was locked, want it to fail")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Pool took %v to fail, want it to give up after truncateTimeout", elapsed)
	}
}
