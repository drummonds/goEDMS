package database

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/drummonds/godocs/config"
	"github.com/oklog/ulid/v2"
)

func TestPglikeDatabase(t *testing.T) {
	if Logger == nil {
		Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	db := NewRepository(config.ServerConfig{DatabaseType: "sqlite", DatabaseDbname: ":memory:"})
	defer db.Close()

	t.Log("pglike SQLite database setup successfully")

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

		err := db.SaveDocument(doc)
		if err != nil {
			t.Fatalf("Failed to save document: %v", err)
		}

		if doc.ID == 0 {
			t.Error("Document ID was not set after save")
		}

		retrieved, err := db.GetDocumentByID(doc.ID)
		if err != nil {
			t.Fatalf("Failed to get document by ID: %v", err)
		}

		if retrieved.Name != doc.Name {
			t.Errorf("Expected name %s, got %s", doc.Name, retrieved.Name)
		}

		retrievedByULID, err := db.GetDocumentByULID(doc.ULID.String())
		if err != nil {
			t.Fatalf("Failed to get document by ULID: %v", err)
		}

		if retrievedByULID.ID != doc.ID {
			t.Errorf("Expected ID %d, got %d", doc.ID, retrievedByULID.ID)
		}
	})

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
	})

	t.Run("Create and retrieve job", func(t *testing.T) {
		job, err := db.CreateJob(JobTypeIngestion, "Test ingestion job")
		if err != nil {
			t.Fatalf("Failed to create job: %v", err)
		}

		if job.ID.String() == "" {
			t.Error("Job ID was not set after create")
		}

		retrievedJob, err := db.GetJob(job.ID)
		if err != nil {
			t.Fatalf("Failed to get job: %v", err)
		}

		if retrievedJob.Message != job.Message {
			t.Errorf("Expected message %s, got %s", job.Message, retrievedJob.Message)
		}

		err = db.UpdateJobProgress(job.ID, 50, "Processing files")
		if err != nil {
			t.Fatalf("Failed to update job progress: %v", err)
		}

		err = db.CompleteJob(job.ID, `{"processed": 10}`)
		if err != nil {
			t.Fatalf("Failed to complete job: %v", err)
		}

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
	})

	t.Run("Word frequency operations", func(t *testing.T) {
		doc := &Document{
			Name:         "wordtest.pdf",
			Path:         "/tmp/wordtest.pdf",
			IngressTime:  time.Now(),
			Folder:       "/tmp",
			Hash:         "wordtest123",
			ULID:         ulid.Make(),
			DocumentType: ".pdf",
			FullText:     "test word test word test another word",
		}

		err := db.SaveDocument(doc)
		if err != nil {
			t.Fatalf("Failed to save document: %v", err)
		}

		err = db.UpdateWordFrequencies(doc.ULID.String())
		if err != nil {
			t.Fatalf("Failed to update word frequencies: %v", err)
		}

		words, err := db.GetTopWords(10)
		if err != nil {
			t.Fatalf("Failed to get top words: %v", err)
		}

		if len(words) == 0 {
			t.Error("Expected some words, got none")
		}

		metadata, err := db.GetWordCloudMetadata()
		if err != nil {
			t.Fatalf("Failed to get word cloud metadata: %v", err)
		}

		if metadata.Version < 1 {
			t.Errorf("Expected version >= 1, got %d", metadata.Version)
		}
	})

	t.Run("Search documents", func(t *testing.T) {
		doc := &Document{
			Name:         "searchtest.pdf",
			Path:         "/tmp/searchtest.pdf",
			IngressTime:  time.Now(),
			Folder:       "/tmp",
			Hash:         "searchtest123",
			ULID:         ulid.Make(),
			DocumentType: ".pdf",
			FullText:     "This document contains searchable content about databases",
		}

		err := db.SaveDocument(doc)
		if err != nil {
			t.Fatalf("Failed to save document: %v", err)
		}

		results, err := db.SearchDocuments("database")
		if err != nil {
			t.Fatalf("Failed to search documents: %v", err)
		}

		if len(results) == 0 {
			t.Error("Expected to find at least one document, got none")
		}
	})

	t.Run("Tag operations", func(t *testing.T) {
		tag := &Tag{
			Name:  "TestTag",
			Color: "#ff0000",
		}

		err := db.CreateTag(tag)
		if err != nil {
			t.Fatalf("Failed to create tag: %v", err)
		}

		if tag.ID == 0 {
			t.Error("Tag ID was not set after create")
		}

		tags, err := db.GetAllTags()
		if err != nil {
			t.Fatalf("Failed to get all tags: %v", err)
		}

		// Should have predefined tags from migrations + our test tag
		if len(tags) == 0 {
			t.Error("Expected at least one tag")
		}

		// Test adding tag to document
		doc, err := db.GetDocumentByPath("/tmp/test.pdf")
		if err != nil {
			t.Fatalf("Failed to get document: %v", err)
		}

		err = db.AddTagToDocument(doc.ID, tag.ID)
		if err != nil {
			t.Fatalf("Failed to add tag to document: %v", err)
		}

		docTags, err := db.GetTagsForDocument(doc.ID)
		if err != nil {
			t.Fatalf("Failed to get tags for document: %v", err)
		}

		found := false
		for _, t2 := range docTags {
			if t2.Name == "TestTag" {
				found = true
			}
		}
		if !found {
			t.Error("Expected to find TestTag on document")
		}
	})

	t.Run("Saved search operations", func(t *testing.T) {
		searches, err := db.GetAllSavedSearches()
		if err != nil {
			t.Fatalf("Failed to get saved searches: %v", err)
		}

		// Should have 2 default searches from migration 007
		if len(searches) < 2 {
			t.Errorf("Expected at least 2 default saved searches, got %d", len(searches))
		}
	})
}
