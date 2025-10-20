package database

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/stapelberg/postgrestest"
)

// EphemeralPostgresDB implements DBInterface using ephemeral PostgreSQL
type EphemeralPostgresDB struct {
	*PostgresDB
	server *postgrestest.Server
}

// SetupEphemeralPostgresDatabase creates an ephemeral PostgreSQL instance
func SetupEphemeralPostgresDatabase() (*EphemeralPostgresDB, error) {
	Logger.Info("Starting ephemeral PostgreSQL server...")

	ctx := context.Background()

	// Start the ephemeral PostgreSQL server
	// Uses a temporary directory by default for simplicity
	pgt, err := postgrestest.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start ephemeral postgres: %w", err)
	}

	// Get the default database DSN
	defaultDSN := pgt.DefaultDatabase()
	Logger.Info("Ephemeral PostgreSQL server started", "dsn", defaultDSN)

	// Create a new database for the application
	goedmsDSN, err := pgt.CreateDatabase(ctx)
	if err != nil {
		pgt.Cleanup()
		return nil, fmt.Errorf("failed to create goedms database: %w", err)
	}

	Logger.Info("Created ephemeral database", "dsn", goedmsDSN)

	// Connect to the new database
	db, err := sql.Open("postgres", goedmsDSN)
	if err != nil {
		pgt.Cleanup()
		return nil, fmt.Errorf("failed to open goedms database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		pgt.Cleanup()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	Logger.Info("Connected to ephemeral PostgreSQL database successfully")

	// Run migrations
	if err := runPostgresMigrations(db); err != nil {
		pgt.Cleanup()
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &EphemeralPostgresDB{
		PostgresDB: &PostgresDB{
			db:         db,
			isEmbedded: true, // Mark as ephemeral
		},
		server: pgt,
	}, nil
}

// Close closes the database connection and cleans up the ephemeral server
func (e *EphemeralPostgresDB) Close() error {
	if e.PostgresDB != nil && e.PostgresDB.db != nil {
		if err := e.PostgresDB.db.Close(); err != nil {
			Logger.Warn("Failed to close database connection", "error", err)
		}
	}

	if e.server != nil {
		Logger.Info("Cleaning up ephemeral PostgreSQL server...")
		e.server.Cleanup()
		Logger.Info("Ephemeral PostgreSQL server cleaned up")
	}

	return nil
}
