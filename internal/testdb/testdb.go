// Package testdb provides a real Postgres for integration tests.
//
// One postgres:18-alpine container is started per test binary on first use and
// removed by the testcontainers reaper (Ryuk) when the binary exits.
package testdb

import (
	"context"
	"sync"
	"testing"

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

// Pool returns a pool connected to the shared test database with all
// migrations applied and the jobs table emptied (identity restarted).
func Pool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	once.Do(func() { pool, initErr = start(ctx) })
	if initErr != nil {
		t.Fatalf("testdb: start postgres: %v", initErr)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE jobs RESTART IDENTITY"); err != nil {
		t.Fatalf("testdb: truncate jobs: %v", err)
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
