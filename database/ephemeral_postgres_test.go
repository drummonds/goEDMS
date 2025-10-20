package database

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stapelberg/postgrestest"
)

func TestEphemeralPostgres(t *testing.T) {
	// Setup logger for test
	Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	t.Log("Starting ephemeral PostgreSQL test...")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Try starting ephemeral PostgreSQL with minimal options
	t.Log("Attempting to start postgrestest server...")
	pgt, err := postgrestest.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start ephemeral postgres: %v", err)
	}
	defer pgt.Cleanup()

	t.Log("Ephemeral PostgreSQL server started successfully!")

	// Get the default database DSN
	defaultDSN := pgt.DefaultDatabase()
	t.Logf("Default database DSN: %s", defaultDSN)

	// Try connecting to it
	db, err := sql.Open("postgres", defaultDSN)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Test the connection
	if err := db.Ping(); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	t.Log("Successfully connected to ephemeral PostgreSQL!")

	// Create a test table
	_, err = db.Exec(`CREATE TABLE test_table (id SERIAL PRIMARY KEY, name VARCHAR(100))`)
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}

	// Insert test data
	_, err = db.Exec(`INSERT INTO test_table (name) VALUES ('test')`)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Query test data
	var name string
	err = db.QueryRow(`SELECT name FROM test_table WHERE id = 1`).Scan(&name)
	if err != nil {
		t.Fatalf("Failed to query test data: %v", err)
	}

	if name != "test" {
		t.Fatalf("Expected name 'test', got '%s'", name)
	}

	t.Log("Ephemeral PostgreSQL test completed successfully!")
}

func TestSetupEphemeralPostgresDatabase(t *testing.T) {
	// Setup logger for test
	Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	t.Log("Testing SetupEphemeralPostgresDatabase function...")

	ephemeralDB, err := SetupEphemeralPostgresDatabase()
	if err != nil {
		t.Fatalf("Failed to setup ephemeral postgres database: %v", err)
	}
	defer ephemeralDB.Close()

	t.Log("Ephemeral database setup successfully!")

	// Test that we can use the database
	doc := &Document{
		Name:         "test.pdf",
		Path:         "/test/test.pdf",
		IngressTime:  time.Now(),
		Folder:       "test",
		Hash:         "testhash123",
		DocumentType: "pdf",
		FullText:     "This is test content",
	}

	// Generate ULID for the document
	doc.ULID = ulid.Make()

	// Try to save a document
	err = ephemeralDB.PostgresDB.SaveDocument(doc)
	if err != nil {
		t.Fatalf("Failed to save document: %v", err)
	}

	t.Logf("Document saved with ID: %d", doc.StormID)

	// Try to retrieve the document
	retrievedDoc, err := ephemeralDB.PostgresDB.GetDocumentByID(doc.StormID)
	if err != nil {
		t.Fatalf("Failed to retrieve document: %v", err)
	}

	if retrievedDoc.Name != doc.Name {
		t.Fatalf("Expected document name '%s', got '%s'", doc.Name, retrievedDoc.Name)
	}

	t.Log("Successfully saved and retrieved document from ephemeral database!")
}
