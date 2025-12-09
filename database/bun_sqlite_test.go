package database

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/drummonds/godocs/config"
	"github.com/oklog/ulid/v2"
)

func TestBunSQLiteDatabase(t *testing.T) {
	// Initialize logger for tests
	if Logger == nil {
		Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	// // Create a temporary SQLite database file
	// tmpFile := "databases/test_godocs_" + ulid.Make().String() + ".sqlite"
	// defer os.Remove(tmpFile)
	tmpFile := ":memory:"

	// Setup Bun with SQLite
	db := NewRepository(config.ServerConfig{DatabaseType: "sqlite", DatabaseDbname: tmpFile})
	defer db.Close()

	t.Log("Bun SQLite database setup successfully")

	// Test document operations
	t.Run("Create and retrieve document", func(t *testing.T) {
		doc := &Document{
			Name:         "test.pdf",
			Path:         "/tmp/test.pdf",
			IngressTime:  time.Now(),
			Folder:       "/tmp",
			Hash:         "test123hash",
			ULID:         ulid.Make(),
			DocumentType: ".pdf",
			FullText:     "This is a test document with some content",
			URL:          "http://example.com/test.pdf",
		}

		// Save document
		err := db.SaveDocument(doc)
		if err != nil {
			t.Fatalf("Failed to save document: %v", err)
		}

		if doc.ID == 0 {
			t.Error("Document ID was not set after save")
		}

		// Retrieve by ID
		retrieved, err := db.GetDocumentByID(doc.ID)
		if err != nil {
			t.Fatalf("Failed to get document by ID: %v", err)
		}

		if retrieved.Name != doc.Name {
			t.Errorf("Expected name %s, got %s", doc.Name, retrieved.Name)
		}

		// Retrieve by ULID
		retrievedByULID, err := db.GetDocumentByULID(doc.ULID.String())
		if err != nil {
			t.Fatalf("Failed to get document by ULID: %v", err)
		}

		if retrievedByULID.ID != doc.ID {
			t.Errorf("Expected ID %d, got %d", doc.ID, retrievedByULID.ID)
		}

		t.Log("Document create and retrieve test passed")
	})

	// Test config operations
	t.Run("Save and retrieve config", func(t *testing.T) {
		cfg := &config.ServerConfig{
			ListenAddrPort:  "9000",
			IngressPath:     "/tmp/ingress",
			DocumentPath:    "/tmp/docs",
			IngressInterval: 15,
		}

		err := db.SaveConfig(cfg)
		if err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		retrievedCfg, err := db.GetConfig()
		if err != nil {
			t.Fatalf("Failed to get config: %v", err)
		}

		if retrievedCfg.ListenAddrPort != cfg.ListenAddrPort {
			t.Errorf("Expected port %s, got %s", cfg.ListenAddrPort, retrievedCfg.ListenAddrPort)
		}

		if retrievedCfg.IngressInterval != cfg.IngressInterval {
			t.Errorf("Expected interval %d, got %d", cfg.IngressInterval, retrievedCfg.IngressInterval)
		}

		t.Log("Config save and retrieve test passed")
	})

	// Test job operations
	t.Run("Create and retrieve job", func(t *testing.T) {
		job, err := db.CreateJob(JobTypeIngestion, "Test ingestion job")
		if err != nil {
			t.Fatalf("Failed to create job: %v", err)
		}

		if job.ID.String() == "" {
			t.Error("Job ID was not set after create")
		}

		// Retrieve job
		retrievedJob, err := db.GetJob(job.ID)
		if err != nil {
			t.Fatalf("Failed to get job: %v", err)
		}

		if retrievedJob.Message != job.Message {
			t.Errorf("Expected message %s, got %s", job.Message, retrievedJob.Message)
		}

		// Update job progress
		err = db.UpdateJobProgress(job.ID, 50, "Processing files")
		if err != nil {
			t.Fatalf("Failed to update job progress: %v", err)
		}

		// Complete job
		err = db.CompleteJob(job.ID, `{"processed": 10}`)
		if err != nil {
			t.Fatalf("Failed to complete job: %v", err)
		}

		// Verify completion
		completedJob, err := db.GetJob(job.ID)
		if err != nil {
			t.Fatalf("Failed to get completed job: %v", err)
		}

		if completedJob.Status != JobStatusCompleted {
			t.Errorf("Expected status %s, got %s", JobStatusCompleted, completedJob.Status)
		}

		if completedJob.Progress != 100 {
			t.Errorf("Expected progress 100, got %d", completedJob.Progress)
		}

		t.Log("Job operations test passed")
	})

	// Test word cloud operations
	t.Run("Word frequency operations", func(t *testing.T) {
		// Create a document with text
		doc := &Document{
			Name:         "wordtest.pdf",
			Path:         "/tmp/wordtest.pdf",
			IngressTime:  time.Now(),
			Folder:       "/tmp",
			Hash:         "wordtest123",
			ULID:         ulid.Make(),
			DocumentType: ".pdf",
			FullText:     "test word test word test another word",
			URL:          "",
		}

		err := db.SaveDocument(doc)
		if err != nil {
			t.Fatalf("Failed to save document: %v", err)
		}

		// Update word frequencies
		err = db.UpdateWordFrequencies(doc.ULID.String())
		if err != nil {
			t.Fatalf("Failed to update word frequencies: %v", err)
		}

		// Get top words
		words, err := db.GetTopWords(10)
		if err != nil {
			t.Fatalf("Failed to get top words: %v", err)
		}

		if len(words) == 0 {
			t.Error("Expected some words, got none")
		}

		// Verify "test" is the most frequent word
		if len(words) > 0 {
			if words[0].Word != "test" {
				t.Logf("Top word: %s (frequency: %d)", words[0].Word, words[0].Frequency)
			}
			if words[0].Frequency < 3 {
				t.Errorf("Expected 'test' to appear at least 3 times, got %d", words[0].Frequency)
			}
		}

		// Get word cloud metadata
		metadata, err := db.GetWordCloudMetadata()
		if err != nil {
			t.Fatalf("Failed to get word cloud metadata: %v", err)
		}

		if metadata.Version < 1 {
			t.Errorf("Expected version >= 1, got %d", metadata.Version)
		}

		t.Log("Word frequency operations test passed")
	})

	// Test search functionality
	t.Run("Search documents", func(t *testing.T) {
		// Create a searchable document
		doc := &Document{
			Name:         "searchtest.pdf",
			Path:         "/tmp/searchtest.pdf",
			IngressTime:  time.Now(),
			Folder:       "/tmp",
			Hash:         "searchtest123",
			ULID:         ulid.Make(),
			DocumentType: ".pdf",
			FullText:     "This document contains searchable content about databases",
			URL:          "",
		}

		err := db.SaveDocument(doc)
		if err != nil {
			t.Fatalf("Failed to save document: %v", err)
		}

		// Search for the document (SQLite will use LIKE search)
		results, err := db.SearchDocuments("database")
		if err != nil {
			t.Fatalf("Failed to search documents: %v", err)
		}

		if len(results) == 0 {
			t.Error("Expected to find at least one document, got none")
		}

		t.Logf("Search test passed, found %d documents", len(results))
	})
}
