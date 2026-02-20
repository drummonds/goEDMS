//go:build !js

package database

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/stapelberg/postgrestest"
)

// EphemeralPostgresDB wraps PGDB with an ephemeral PostgreSQL server for testing
type EphemeralPostgresDB struct {
	*PGDB
	server *postgrestest.Server
}

// SetupEphemeralPostgresDatabase creates an ephemeral PostgreSQL instance
func SetupEphemeralPostgresDatabase() (*PGDB, error) {
	Logger.Info("Starting ephemeral PostgreSQL server...")

	ctx := context.Background()

	pgt, err := postgrestest.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start ephemeral postgres: %w", err)
	}

	godocsDSN, err := pgt.CreateDatabase(ctx)
	if err != nil {
		pgt.Cleanup()
		return nil, fmt.Errorf("failed to create godocs database: %w", err)
	}

	Logger.Info("Created ephemeral database", "dsn", godocsDSN)

	db, err := sql.Open("postgres", godocsDSN)
	if err != nil {
		pgt.Cleanup()
		return nil, fmt.Errorf("failed to open godocs database: %w", err)
	}

	if err := db.Ping(); err != nil {
		pgt.Cleanup()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	Logger.Info("Connected to ephemeral PostgreSQL database successfully")

	// Run migrations (not pglike - this is real PostgreSQL)
	if err := runMigrations(ctx, db, false); err != nil {
		pgt.Cleanup()
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &PGDB{db: db, isPglike: false}, nil
}
