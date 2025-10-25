package database

import (
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestWordTokenizer(t *testing.T) {
	// Initialize logger
	Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

	tokenizer := NewWordTokenizer()

	t.Run("Basic tokenization", func(t *testing.T) {
		text := "This is a test document with some repeated words. Document processing is important for document management."
		frequencies := tokenizer.TokenizeAndCount(text)

		// "document" appears 3 times
		if frequencies["document"] != 3 {
			t.Errorf("Expected 'document' to appear 3 times, got %d", frequencies["document"])
		}

		// Stop words should be filtered
		if _, exists := frequencies["the"]; exists {
			t.Error("Stop word 'the' should be filtered out")
		}
		if _, exists := frequencies["is"]; exists {
			t.Error("Stop word 'is' should be filtered out")
		}
		if _, exists := frequencies["a"]; exists {
			t.Error("Stop word 'a' should be filtered out")
		}

		// Short words should be filtered
		if _, exists := frequencies["is"]; exists {
			t.Error("Short word 'is' should be filtered out")
		}
	})

	t.Run("Case insensitivity", func(t *testing.T) {
		text := "Document DOCUMENT document"
		frequencies := tokenizer.TokenizeAndCount(text)

		if frequencies["document"] != 3 {
			t.Errorf("Expected case-insensitive count of 3, got %d", frequencies["document"])
		}
	})

	t.Run("Number filtering", func(t *testing.T) {
		text := "Test 123 document 456 789"
		frequencies := tokenizer.TokenizeAndCount(text)

		if _, exists := frequencies["123"]; exists {
			t.Error("Numbers should be filtered out")
		}
		if frequencies["test"] != 1 {
			t.Error("'test' should be counted")
		}
		if frequencies["document"] != 1 {
			t.Error("'document' should be counted")
		}
	})

	t.Run("Hyphenated words", func(t *testing.T) {
		text := "full-text well-known state-of-the-art"
		frequencies := tokenizer.TokenizeAndCount(text)

		if frequencies["full-text"] != 1 {
			t.Error("Hyphenated word 'full-text' should be counted")
		}
		if frequencies["well-known"] != 1 {
			t.Error("Hyphenated word 'well-known' should be counted")
		}
	})

	t.Run("Minimum word length", func(t *testing.T) {
		text := "a to be or it in on at by"
		frequencies := tokenizer.TokenizeAndCount(text)

		// All should be filtered (too short or stop words)
		if len(frequencies) != 0 {
			t.Errorf("Expected all short words to be filtered, got %d words", len(frequencies))
		}
	})
}

func TestWordCloudIntegration(t *testing.T) {
	// Initialize logger
	Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Setup ephemeral database for testing
	postgresDB, err := SetupPostgresDatabase("")
	if err != nil {
		t.Fatalf("Failed to setup ephemeral database: %v", err)
	}
	defer postgresDB.Close()

	t.Run("Full recalculation", func(t *testing.T) {
		// This test verifies the full recalculation works
		err := postgresDB.RecalculateAllWordFrequencies()
		if err != nil {
			t.Fatalf("RecalculateAllWordFrequencies failed: %v", err)
		}

		// Get metadata
		metadata, err := postgresDB.GetWordCloudMetadata()
		if err != nil {
			t.Fatalf("GetWordCloudMetadata failed: %v", err)
		}

		t.Logf("Metadata: %+v", metadata)
	})

	t.Run("Get top words", func(t *testing.T) {
		// Get top 10 words
		words, err := postgresDB.GetTopWords(10)
		if err != nil {
			t.Fatalf("GetTopWords failed: %v", err)
		}

		t.Logf("Got %d top words", len(words))
		for i, word := range words {
			t.Logf("  %d. %s: %d occurrences", i+1, word.Word, word.Frequency)
		}
	})
}

func TestWordCloudWithDocuments(t *testing.T) {
	// Initialize logger
	Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Setup ephemeral database
	postgresDB, err := SetupPostgresDatabase("")
	if err != nil {
		t.Fatalf("Failed to setup ephemeral database: %v", err)
	}
	defer postgresDB.Close()

	// Create test documents
	testDocs := []struct {
		name    string
		content string
	}{
		{
			name:    "Invoice_2024.pdf",
			content: "This is an invoice for services rendered. Invoice total is $1500. Please pay invoice promptly.",
		},
		{
			name:    "Contract.pdf",
			content: "This contract outlines the agreement between parties. Contract terms are binding.",
		},
		{
			name:    "Report.txt",
			content: "Annual report showing financial performance. Report includes charts and graphs.",
		},
	}

	// Insert documents
	for i, doc := range testDocs {
		ulid, _ := CalculateUUID(time.Now().Add(time.Duration(i) * time.Millisecond))
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

		err := postgresDB.SaveDocument(newDoc)
		if err != nil {
			t.Fatalf("Failed to save document: %v", err)
		}
	}

	// Recalculate word frequencies
	err = postgresDB.RecalculateAllWordFrequencies()
	if err != nil {
		t.Fatalf("RecalculateAllWordFrequencies failed: %v", err)
	}

	// Get top words
	words, err := postgresDB.GetTopWords(20)
	if err != nil {
		t.Fatalf("GetTopWords failed: %v", err)
	}

	t.Logf("Found %d unique words", len(words))

	// Verify specific words
	wordMap := make(map[string]int)
	for _, w := range words {
		wordMap[w.Word] = w.Frequency
	}

	// "invoice" should appear 3 times (3 in content)
	if count, exists := wordMap["invoice"]; !exists {
		t.Error("'invoice' should be in word cloud")
	} else if count != 3 {
		t.Errorf("'invoice' should appear 3 times, got %d", count)
	}

	// "contract" should appear 3 times (2 in content + 1 in filename "Contract.pdf")
	if count, exists := wordMap["contract"]; !exists {
		t.Error("'contract' should be in word cloud")
	} else if count != 3 {
		t.Errorf("'contract' should appear 3 times, got %d", count)
	}

	// "report" should appear 3 times (2 in content + 1 in filename "Report.txt")
	if count, exists := wordMap["report"]; !exists {
		t.Error("'report' should be in word cloud")
	} else if count != 3 {
		t.Errorf("'report' should appear 3 times, got %d", count)
	}

	// Stop words should not appear
	stopWordExamples := []string{"this", "is", "the", "for", "and"}
	for _, stopWord := range stopWordExamples {
		if _, exists := wordMap[stopWord]; exists {
			t.Errorf("Stop word '%s' should not be in word cloud", stopWord)
		}
	}

	t.Logf("Top 10 words:")
	for i := 0; i < 10 && i < len(words); i++ {
		t.Logf("  %d. %s: %d", i+1, words[i].Word, words[i].Frequency)
	}
}
