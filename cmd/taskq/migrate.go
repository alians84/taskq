package main

import (
	"context"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/alians84/taskq/migrations"
)

func runMigrate(ctx context.Context) error {
	_, log, pool, err := setup(ctx)
	if err != nil {
		return err
	}
	defer pool.Close()

	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	if err := migrations.Up(ctx, db); err != nil {
		return err
	}
	log.InfoContext(ctx, "migrations applied")
	return nil
}
