// Package testdb provides a real Postgres for integration tests.
//
// One postgres:18-alpine container is started per test binary on first use and
// removed by the testcontainers reaper (Ryuk) when the binary exits.
package testdb

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/alians84/taskq/migrations"
)

var (
	once    sync.Once
	pool    *pgxpool.Pool
	initErr error
)

// truncateTimeout bounds the per-call cleanup, so a test that leaked an open
// transaction makes the next test fail fast instead of hanging.
var truncateTimeout = 10 * time.Second

// Pool returns a pool connected to the shared test database with all
// migrations applied and the jobs table emptied (identity restarted).
//
// Every call wipes the table, so tests using Pool must not run with t.Parallel.
func Pool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	once.Do(func() { pool, initErr = start(ctx) })
	if initErr != nil {
		t.Fatalf("testdb: start postgres: %v", initErr)
	}
	truncCtx, cancel := context.WithTimeout(t.Context(), truncateTimeout)
	defer cancel()
	if _, err := pool.Exec(truncCtx, "TRUNCATE jobs RESTART IDENTITY"); err != nil {
		t.Fatalf("testdb: truncate jobs (is a transaction from a previous test still open?): %v", err)
	}
	return pool
}

func start(ctx context.Context) (*pgxpool.Pool, error) {
	ctr, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("taskq"),
		postgres.WithUsername("taskq"),
		postgres.WithPassword("taskq"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, err
	}
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, err
	}
	p, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	db := stdlib.OpenDBFromPool(p)
	defer db.Close()
	if err := migrations.Up(ctx, db); err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}
