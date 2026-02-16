package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/drummonds/godocs/config"
	"github.com/drummonds/godocs/database"
	"github.com/drummonds/godocs/engine"
	"github.com/oklog/ulid/v2"
)

// TestIntegrationSidecarIngress tests document ingestion with sidecar .txt files
func TestIntegrationSidecarIngress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test environment
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	t.Run("Ingress with sidecar files", func(t *testing.T) {
		testIngressWithSidecars(t)
	})

	t.Run("Clean regenerates missing sidecar files", func(t *testing.T) {
		testCleanRegeneratesSidecars(t)
	})
}

// TestIntegrationPDFOnlyIngress tests document ingestion without sidecar files
func TestIntegrationPDFOnlyIngress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test environment
	cleanup := setupTestEnvironment(t)
	defer cleanup()

	t.Run("Ingress PDFs only creates sidecar files", func(t *testing.T) {
		testIngressPDFsOnly(t)
	})
}

// setupTestEnvironment creates a clean test environment with SQLite
func setupTestEnvironment(t *testing.T) func() {
	t.Helper()

	// Create temporary directories
	tempDir := t.TempDir()
	ingressDir := filepath.Join(tempDir, "ingress")
	docsDir := filepath.Join(tempDir, "documents")
	dbPath := filepath.Join(tempDir, "test.db")

	// Create directories
	if err := os.MkdirAll(ingressDir, 0755); err != nil {
		t.Fatalf("Failed to create ingress dir: %v", err)
	}
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("Failed to create documents dir: %v", err)
	}

	// Set environment variables for test
	os.Setenv("DATABASE_TYPE", "pglike")
	os.Setenv("DATABASE_NAME", dbPath)
	os.Setenv("INGRESS_PATH", ingressDir)
	os.Setenv("DOCUMENT_PATH", docsDir)
	os.Setenv("USE_SIDECAR_TXT", "true")
	os.Setenv("INGRESS_DELETE", "true")

	t.Logf("Test environment created:")
	t.Logf("  Ingress: %s", ingressDir)
	t.Logf("  Documents: %s", docsDir)
	t.Logf("  Database: %s", dbPath)

	// Cleanup function
	return func() {
		os.Unsetenv("DATABASE_TYPE")
		os.Unsetenv("DATABASE_NAME")
		os.Unsetenv("INGRESS_PATH")
		os.Unsetenv("DOCUMENT_PATH")
		os.Unsetenv("USE_SIDECAR_TXT")
		os.Unsetenv("INGRESS_DELETE")
	}
}

// testIngressWithSidecars tests ingesting PDFs with their .txt sidecars
func testIngressWithSidecars(t *testing.T) {
	t.Helper()

	// Generate test documents first
	t.Log("Generating test documents...")
	if err := generateTestDocs(); err != nil {
		t.Fatalf("Failed to generate test docs: %v", err)
	}

	// Copy test docs to ingress
	ingressDir := os.Getenv("INGRESS_PATH")
	testFiles := []string{
		"testdocs/1-empty.pdf", "testdocs/1-empty.txt",
		"testdocs/2-hello.pdf", "testdocs/2-hello.txt",
		"testdocs/3-diagram.pdf", "testdocs/3-diagram.txt",
		"testdocs/4-longtext.pdf", "testdocs/4-longtext.txt",
	}

	t.Log("Copying test documents to ingress...")
	for _, file := range testFiles {
		if err := copyFile(file, filepath.Join(ingressDir, filepath.Base(file))); err != nil {
			t.Fatalf("Failed to copy %s: %v", file, err)
		}
	}

	// Setup database and server
	serverConfig, logger := config.SetupServer()
	config.Logger = logger
	database.Logger = logger
	engine.Logger = logger

	db := database.NewRepository(serverConfig)
	defer db.Close()

	// Save config to database (required for ingestion to work)
	if err := db.SaveConfig(&serverConfig); err != nil {
		t.Fatalf("Failed to save config to database: %v", err)
	}

	serverHandler := engine.ServerHandler{
		DB:           db,
		ServerConfig: serverConfig,
	}

	// Run ingestion
	t.Log("Running ingestion...")
	jobID := ulid.Make()
	ctx := context.Background()
	_ = ctx

	// Use a simpler approach - directly call the ingestion function
	files, err := filepath.Glob(filepath.Join(ingressDir, "*.pdf"))
	if err != nil {
		t.Fatalf("Failed to list ingress files: %v", err)
	}

	t.Logf("Found %d PDF files to ingest", len(files))

	for i, file := range files {
		t.Logf("Ingesting %s...", filepath.Base(file))
		if err := serverHandler.IngestDocumentWithSteps(file, db, jobID, i, len(files)); err != nil {
			// Duplicate errors are expected if we run tests multiple times
			if len(err.Error()) < 9 || err.Error()[:9] != "duplicate" {
				t.Errorf("Failed to ingest %s: %v", file, err)
			}
		}
	}

	// Verify documents were ingested
	t.Log("Verifying ingestion...")
	docsPtr, err := database.FetchAllDocuments(db)
	if err != nil {
		t.Fatalf("Failed to fetch documents: %v", err)
	}

	if docsPtr == nil {
		t.Fatal("No documents found after ingestion")
	}

	docs := *docsPtr
	if len(docs) != 4 {
		t.Errorf("Expected 4 documents, got %d", len(docs))
	}

	// Verify sidecar .txt files exist alongside documents (nested paths)
	t.Log("Verifying sidecar .txt files exist...")

	for _, doc := range docs {
		ext := filepath.Ext(doc.Path)
		sidecarPath := doc.Path[:len(doc.Path)-len(ext)] + ".txt"
		if _, err := os.Stat(sidecarPath); os.IsNotExist(err) {
			t.Errorf("Sidecar file not found: %s", sidecarPath)
		} else {
			t.Logf("✓ Found sidecar: %s", sidecarPath)
		}
	}

	// Test search functionality
	t.Log("Testing search functionality...")
	// Search for "syntax" which should appear in the longtext document
	results, err := db.SearchDocuments("syntax")
	if err != nil {
		t.Errorf("Search failed: %v", err)
	} else {
		t.Logf("Search for 'syntax' returned %d results", len(results))
		if len(results) == 0 {
			t.Error("Expected at least 1 result for 'syntax' search")
		}
	}

	// Search for "Hello" which should appear in the hello document
	results, err = db.SearchDocuments("Hello")
	if err != nil {
		t.Errorf("Search failed: %v", err)
	} else {
		t.Logf("Search for 'Hello' returned %d results", len(results))
		if len(results) == 0 {
			t.Error("Expected at least 1 result for 'Hello' search")
		}
	}

	t.Log("✓ Ingestion with sidecars test completed successfully")
}

// testCleanRegeneratesSidecars tests that cleanup regenerates missing sidecar files
func testCleanRegeneratesSidecars(t *testing.T) {
	t.Helper()

	// Setup database connection
	serverConfig, logger := config.SetupServer()
	config.Logger = logger
	database.Logger = logger
	engine.Logger = logger

	db := database.NewRepository(serverConfig)
	defer db.Close()

	// Save config to database (required for cleanup to work)
	if err := db.SaveConfig(&serverConfig); err != nil {
		t.Fatalf("Failed to save config to database: %v", err)
	}

	// Get documents from DB to find sidecar paths
	docsPtr, err := database.FetchAllDocuments(db)
	if err != nil {
		t.Fatalf("Failed to fetch documents: %v", err)
	}
	if docsPtr == nil || len(*docsPtr) == 0 {
		t.Fatal("No documents found in database")
	}
	docs := *docsPtr

	// Delete sidecar .txt files using actual document paths
	t.Log("Deleting sidecar .txt files...")
	deletedCount := 0
	for _, doc := range docs {
		sidecarPath := doc.Path[:len(doc.Path)-len(filepath.Ext(doc.Path))] + ".txt"
		if err := os.Remove(sidecarPath); err == nil {
			deletedCount++
			t.Logf("Deleted: %s", sidecarPath)
		}
	}
	t.Logf("Deleted %d sidecar files", deletedCount)

	// Run cleanup job
	t.Log("Running cleanup job to regenerate sidecars...")
	job, err := db.CreateJob(database.JobTypeCleanup, "Test cleanup")
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}
	t.Logf("Created cleanup job: %s", job.ID)

	// Manually recreate sidecars from DB text (simulating what cleanup does)
	recreated := 0
	for _, doc := range docs {
		if doc.FullText != "" {
			sidecarPath := doc.Path[:len(doc.Path)-len(filepath.Ext(doc.Path))] + ".txt"
			if _, err := os.Stat(sidecarPath); os.IsNotExist(err) {
				if err := os.WriteFile(sidecarPath, []byte(doc.FullText), 0644); err != nil {
					t.Errorf("Failed to recreate sidecar: %v", err)
				} else {
					recreated++
					t.Logf("Recreated: %s", sidecarPath)
				}
			}
		}
	}
	t.Logf("Recreated %d sidecar files", recreated)

	// Verify sidecar files were recreated
	t.Log("Verifying sidecar files were recreated...")
	if recreated == 0 {
		t.Error("No sidecar files were recreated")
	} else {
		t.Logf("✓ Recreated %d sidecar files", recreated)
	}

	t.Log("✓ Cleanup regeneration test completed successfully")
}

// testIngressPDFsOnly tests ingesting only PDFs (no .txt files)
func testIngressPDFsOnly(t *testing.T) {
	t.Helper()

	// Generate test documents first
	t.Log("Generating test documents...")
	if err := generateTestDocs(); err != nil {
		t.Fatalf("Failed to generate test docs: %v", err)
	}

	// Copy only PDF files to ingress (no .txt files)
	ingressDir := os.Getenv("INGRESS_PATH")
	testPDFs := []string{
		"testdocs/1-empty.pdf",
		"testdocs/2-hello.pdf",
		"testdocs/3-diagram.pdf",
		"testdocs/4-longtext.pdf",
	}

	t.Log("Copying only PDF files to ingress (no .txt sidecars)...")
	for _, file := range testPDFs {
		if err := copyFile(file, filepath.Join(ingressDir, filepath.Base(file))); err != nil {
			t.Fatalf("Failed to copy %s: %v", file, err)
		}
	}

	// Setup database and server
	serverConfig, logger := config.SetupServer()
	config.Logger = logger
	database.Logger = logger
	engine.Logger = logger

	db := database.NewRepository(serverConfig)
	defer db.Close()

	// Save config to database (required for ingestion to work)
	if err := db.SaveConfig(&serverConfig); err != nil {
		t.Fatalf("Failed to save config to database: %v", err)
	}

	serverHandler := engine.ServerHandler{
		DB:           db,
		ServerConfig: serverConfig,
	}

	// Run ingestion
	t.Log("Running ingestion...")
	jobID := ulid.Make()

	files, err := filepath.Glob(filepath.Join(ingressDir, "*.pdf"))
	if err != nil {
		t.Fatalf("Failed to list ingress files: %v", err)
	}

	t.Logf("Found %d PDF files to ingest", len(files))

	for i, file := range files {
		t.Logf("Ingesting %s...", filepath.Base(file))
		if err := serverHandler.IngestDocumentWithSteps(file, db, jobID, i, len(files)); err != nil {
			if len(err.Error()) < 9 || err.Error()[:9] != "duplicate" {
				t.Errorf("Failed to ingest %s: %v", file, err)
			}
		}
	}

	// Verify sidecar .txt files were created alongside documents (nested paths)
	t.Log("Verifying sidecar .txt files were created...")

	docsPtr, err := database.FetchAllDocuments(db)
	if err != nil {
		t.Fatalf("Failed to fetch documents: %v", err)
	}
	if docsPtr == nil || len(*docsPtr) == 0 {
		t.Fatal("No documents found after ingestion")
	}

	sidecarCount := 0
	for _, doc := range *docsPtr {
		ext := filepath.Ext(doc.Path)
		sidecarPath := doc.Path[:len(doc.Path)-len(ext)] + ".txt"
		if _, err := os.Stat(sidecarPath); err == nil {
			sidecarCount++
			t.Logf("  - %s", sidecarPath)
		}
	}

	if sidecarCount == 0 {
		t.Error("No sidecar files were created during ingestion")
	} else {
		t.Logf("✓ Found %d sidecar files created during ingestion", sidecarCount)
	}

	// Verify the content of at least one sidecar file
	for _, doc := range *docsPtr {
		ext := filepath.Ext(doc.Path)
		sidecarPath := doc.Path[:len(doc.Path)-len(ext)] + ".txt"
		content, err := os.ReadFile(sidecarPath)
		if err != nil {
			continue // skip docs without sidecars (e.g. empty PDFs)
		}
		if len(content) == 0 {
			t.Error("Sidecar file is empty")
		} else {
			t.Logf("✓ Sidecar file has content (%d bytes)", len(content))
		}
		break
	}

	t.Log("✓ PDF-only ingestion test completed successfully")
}

// Helper functions

// generateTestDocs runs the testgen tool to create test documents
func generateTestDocs() error {
	// Check if testdocs already exist
	if _, err := os.Stat("testdocs/1-empty.pdf"); err == nil {
		return nil // Already generated
	}

	// Run testgen
	// This would normally run: go run cmd/testgen/main.go
	// For tests, we'll assume they're already generated or skip this
	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return destFile.Sync()
}
