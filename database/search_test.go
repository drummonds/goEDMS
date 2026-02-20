//go:build !js

package database

import (
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestParseSearchQuery(t *testing.T) {
	t.Run("AfterDate", func(t *testing.T) {
		result := ParseSearchQuery("after:2024-01-15")
		if result.AfterDate == nil {
			t.Fatal("Expected AfterDate to be set")
		}
		expected := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
		if !result.AfterDate.Equal(expected) {
			t.Errorf("Expected AfterDate %v, got %v", expected, *result.AfterDate)
		}
		if result.TextTerms != "" {
			t.Errorf("Expected empty text terms, got %q", result.TextTerms)
		}
	})

	t.Run("BeforeDate", func(t *testing.T) {
		result := ParseSearchQuery("before:2024-12-31")
		if result.BeforeDate == nil {
			t.Fatal("Expected BeforeDate to be set")
		}
		expected := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)
		if !result.BeforeDate.Equal(expected) {
			t.Errorf("Expected BeforeDate %v, got %v", expected, *result.BeforeDate)
		}
	})

	t.Run("DateRange", func(t *testing.T) {
		result := ParseSearchQuery("after:2024-01-01 before:2024-12-31")
		if result.AfterDate == nil || result.BeforeDate == nil {
			t.Fatal("Expected both dates to be set")
		}
		if result.TextTerms != "" {
			t.Errorf("Expected empty text terms, got %q", result.TextTerms)
		}
	})

	t.Run("DateWithTextAndTags", func(t *testing.T) {
		result := ParseSearchQuery("invoice #Finance after:2024-06-01")
		if result.AfterDate == nil {
			t.Fatal("Expected AfterDate to be set")
		}
		if result.TextTerms != "invoice" {
			t.Errorf("Expected text 'invoice', got %q", result.TextTerms)
		}
		if len(result.IncludeTags) != 1 || result.IncludeTags[0] != "Finance" {
			t.Errorf("Expected include tag Finance, got %v", result.IncludeTags)
		}
	})

	t.Run("InvalidDate", func(t *testing.T) {
		result := ParseSearchQuery("after:2024-13-45")
		if result.AfterDate != nil {
			t.Error("Expected invalid date to be ignored")
		}
	})

	t.Run("InvalidDateFormat", func(t *testing.T) {
		result := ParseSearchQuery("after:not-a-date")
		if result.AfterDate != nil {
			t.Error("Expected non-date string to be ignored")
		}
		// "after:not-a-date" won't match the regex, so it stays as text
	})
}

func TestPostgresFullTextSearch(t *testing.T) {
	// Initialize logger
	Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Setup ephemeral database for testing
	postgresDB, err := SetupEphemeralPostgresDatabase()
	if err != nil {
		t.Fatalf("Failed to setup ephemeral database: %v", err)
	}
	defer postgresDB.Close()

	// Create test documents with different content
	testDocs := []struct {
		name     string
		content  string
		expected bool // whether this document should be found for "test invoice" search
	}{
		{"Invoice_2024.pdf", "This is a test invoice for January 2024", true},
		{"Receipt_March.pdf", "Receipt for testing purposes", false},
		{"Invoice_Q1.pdf", "First quarter invoice summary report", true},
		{"Random_Doc.pdf", "This document contains random text about nothing related", false},
		{"Test_Report.pdf", "Testing report with invoice data included", true},
	}

	// Add documents to database
	for i, doc := range testDocs {
		ulid, err := CalculateUUID(time.Now().Add(time.Duration(i) * time.Millisecond))
		if err != nil {
			t.Fatalf("Failed to generate ULID: %v", err)
		}

		newDoc := &Document{
			Name:         doc.name,
			Path:         "/test/" + doc.name,
			Folder:       "/test",
			Hash:         fmt.Sprintf("hash%d", i),
			FullText:     doc.content,
			IngressTime:  time.Now(),
			DocumentType: ".pdf",
			ULID:         ulid,
		}

		err = postgresDB.SaveDocument(newDoc)
		if err != nil {
			t.Fatalf("Failed to save document %s: %v", doc.name, err)
		}
	}

	// Test 1: Single word search
	t.Run("SingleWordSearch", func(t *testing.T) {
		results, err := postgresDB.SearchDocuments("invoice")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 3 {
			t.Errorf("Expected 3 results for 'invoice', got %d", len(results))
			for _, r := range results {
				t.Logf("Found: %s - %s", r.Name, r.FullText)
			}
		}
	})

	// Test 2: Phrase search
	t.Run("PhraseSearch", func(t *testing.T) {
		results, err := postgresDB.SearchDocuments("test invoice")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) == 0 {
			t.Error("Expected at least 1 result for 'test invoice', got 0")
		}

		// Verify the top result contains both words
		if len(results) > 0 {
			found := false
			for _, doc := range results {
				if doc.Name == "Invoice_2024.pdf" {
					found = true
					break
				}
			}
			if !found {
				t.Error("Expected to find 'Invoice_2024.pdf' in results")
			}
		}
	})

	// Test 3: Prefix search
	t.Run("PrefixSearch", func(t *testing.T) {
		results, err := postgresDB.SearchDocuments("invoi")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 3 {
			t.Errorf("Expected 3 results for prefix 'invoi', got %d", len(results))
		}
	})

	// Test 4: No results search
	t.Run("NoResultsSearch", func(t *testing.T) {
		results, err := postgresDB.SearchDocuments("xyz123nonexistent")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected 0 results for nonexistent term, got %d", len(results))
		}
	})

	// Test 5: Empty search term
	t.Run("EmptySearchTerm", func(t *testing.T) {
		results, err := postgresDB.SearchDocuments("")
		if err != nil {
			t.Fatalf("Search failed: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("Expected 0 results for empty search term, got %d", len(results))
		}
	})
}
