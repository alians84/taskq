// Package migrations embeds the SQL schema and applies it with goose.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

// FS holds every migration file of the project.
//
//go:embed *.sql
var FS embed.FS

// Up applies all pending migrations.
func Up(ctx context.Context, db *sql.DB) error {
	provider, err := goose.NewProvider(goose.DialectPostgres, db, FS)
	if err != nil {
		return fmt.Errorf("migrations: new provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrations: up: %w", err)
	}
	return nil
}
