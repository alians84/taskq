//go:build integration

package testdb

import (
	"context"
	"testing"
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
