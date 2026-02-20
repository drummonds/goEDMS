//go:build !js

package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/drummonds/godocs/config"
)

// NewRepository initializes the database based on configuration
func NewRepository(cfg config.ServerConfig) *PGDB {
	// databases dir used by sqlite so might as well make for all
	if _, err := os.Stat("databases"); os.IsNotExist(err) {
		if err := os.Mkdir("databases", os.ModePerm); err != nil {
			Logger.Error("Unable to create folder for databases", "error", err)
			os.Exit(1)
		}
	}

	var (
		sqlDB    *sql.DB
		isPglike bool
		err      error
	)

	dbType := cfg.DatabaseType
	switch dbType {
	case "postgres", "cockroachdb":
		Logger.Info("Initializing postgres database...", "type", dbType)
		userpw := cfg.DatabaseUser
		if cfg.DatabasePassword != "" {
			userpw += fmt.Sprintf(":%s", cfg.DatabasePassword)
		}
		connectionString := fmt.Sprintf("%s://%s@%s:%s/%s?sslmode=%s",
			cfg.DatabaseType, userpw, cfg.DatabaseHost, cfg.DatabasePort, cfg.DatabaseDbname, cfg.DatabaseSslmode)
		Logger.Info("Connection string", "connectionString", connectionString)
		sqlDB, err = sql.Open("postgres", connectionString)
		if err != nil {
			Logger.Error("Failed to open database", "error", err)
			os.Exit(1)
		}

	case "pglike":
		Logger.Info("Initializing go-postgres (pglike) database...", "type", dbType)
		dbName := cfg.DatabaseDbname
		if dbName == "" {
			dbName = "databases/godocs.db"
		}
		sqlDB, err = sql.Open("pglike", dbName)
		if err != nil {
			Logger.Error("Failed to open pglike database", "error", err)
			os.Exit(1)
		}
		isPglike = true

	case "ephemeral":
		Logger.Info("Starting ephemeral PostgreSQL database for development")
		ephDB, err := SetupEphemeralPostgresDatabase()
		if err != nil {
			Logger.Error("Failed to setup ephemeral database", "error", err)
			os.Exit(1)
		}
		return ephDB

	default:
		Logger.Error("Unknown database type", "type", dbType)
		Logger.Info("Supported database types: ephemeral, postgres, cockroachdb, pglike")
		os.Exit(1)
	}

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		Logger.Error("Failed to ping database", "error", err)
		os.Exit(1)
	}
	Logger.Info("Connected to database successfully", "type", dbType)

	// Run migrations
	Logger.Info("Running database migrations...")
	if err := runMigrations(context.Background(), sqlDB, isPglike); err != nil {
		Logger.Error("Failed to run migrations", "error", err)
		os.Exit(1)
	}
	Logger.Info("Database migrations completed successfully")

	return &PGDB{db: sqlDB, isPglike: isPglike}
}
