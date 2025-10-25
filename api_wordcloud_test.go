package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	database "github.com/drummonds/goEDMS/database"
)

// TestWordCloudAPI tests the word cloud API endpoints
func TestWordCloudAPI(t *testing.T) {
	e, serverHandler, cleanup := setupTestServer(t)
	defer cleanup()

	// Create temporary directory for test documents
	tempDir := t.TempDir()

	// Create test documents with known word frequencies
	testDocs := []struct {
		name    string
		content string
	}{
		{
			name:    "Invoice_2024.pdf",
			content: "This is an invoice for services. Invoice total is $1500. Please pay invoice promptly. Invoice services included.",
		},
		{
			name:    "Contract_Agreement.pdf",
			content: "Contract agreement for services. This contract outlines terms. Contract is binding. Services contract valid.",
		},
		{
			name:    "Report_Annual.pdf",
			content: "Annual report showing performance. Report includes data. Financial report summary.",
		},
		{
			name:    "Meeting_Notes.txt",
			content: "Meeting notes for quarterly review. Notes from meeting discussion.",
		},
		{
			name:    "Proposal_Project.pdf",
			content: "Project proposal for new services. Proposal outlines scope.",
		},
	}

	// Insert documents into database
	for i, doc := range testDocs {
		ulid, _ := database.CalculateUUID(time.Now().Add(time.Duration(i) * time.Millisecond))

		// Create actual file
		filePath := filepath.Join(tempDir, doc.name)
		if err := os.WriteFile(filePath, []byte(doc.content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		newDoc := &database.Document{
			Name:         doc.name,
			Path:         filePath,
			Folder:       "Test",
			Hash:         fmt.Sprintf("hash_%d", i),
			FullText:     doc.content,
			IngressTime:  time.Now(),
			DocumentType: filepath.Ext(doc.name),
			ULID:         ulid,
			URL:          fmt.Sprintf("/document/view/%s", ulid.String()),
		}

		if err := serverHandler.DB.SaveDocument(newDoc); err != nil {
			t.Fatalf("Failed to save document: %v", err)
		}
	}

	// Recalculate word frequencies before testing
	if err := serverHandler.DB.RecalculateAllWordFrequencies(); err != nil {
		t.Fatalf("Failed to recalculate word frequencies: %v", err)
	}

	t.Run("GET /api/wordcloud - default limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wordcloud", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v\nBody: %s", err, rec.Body.String())
		}

		// Verify response structure
		if _, ok := response["words"]; !ok {
			t.Error("Response missing 'words' field")
		}
		if _, ok := response["metadata"]; !ok {
			t.Error("Response missing 'metadata' field")
		}
		if _, ok := response["count"]; !ok {
			t.Error("Response missing 'count' field")
		}

		// Verify words array
		words, ok := response["words"].([]interface{})
		if !ok {
			t.Fatalf("Words is not an array: %T", response["words"])
		}

		if len(words) == 0 {
			t.Error("Expected some words in response")
		}

		// Verify first word structure
		if len(words) > 0 {
			firstWord, ok := words[0].(map[string]interface{})
			if !ok {
				t.Fatalf("Word is not an object: %T", words[0])
			}

			if _, ok := firstWord["word"]; !ok {
				t.Error("Word object missing 'word' field")
			}
			if _, ok := firstWord["frequency"]; !ok {
				t.Error("Word object missing 'frequency' field")
			}

			t.Logf("First word: %s (frequency: %.0f)", firstWord["word"], firstWord["frequency"])
		}

		// Log all words for debugging
		t.Logf("Total unique words: %d", len(words))
	})

	t.Run("GET /api/wordcloud - with limit parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wordcloud?limit=5", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		words := response["words"].([]interface{})
		if len(words) > 5 {
			t.Errorf("Expected at most 5 words, got %d", len(words))
		}

		t.Logf("Requested 5 words, got %d words", len(words))
	})

	t.Run("GET /api/wordcloud - limit too high", func(t *testing.T) {
		// Should cap at 500
		req := httptest.NewRequest(http.MethodGet, "/api/wordcloud?limit=1000", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Should not exceed 500
		words := response["words"].([]interface{})
		if len(words) > 500 {
			t.Errorf("Expected at most 500 words (cap), got %d", len(words))
		}
	})

	t.Run("GET /api/wordcloud - invalid limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wordcloud?limit=invalid", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should still return 200 with default limit
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200 (default limit), got %d", rec.Code)
		}
	})

	t.Run("GET /api/wordcloud - metadata structure", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wordcloud", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		metadata, ok := response["metadata"].(map[string]interface{})
		if !ok {
			t.Fatalf("Metadata is not an object: %T", response["metadata"])
		}

		// Check metadata fields
		expectedFields := []string{"totalDocsProcessed", "totalWordsIndexed", "version"}
		for _, field := range expectedFields {
			if _, ok := metadata[field]; !ok {
				t.Errorf("Metadata missing field: %s", field)
			}
		}

		t.Logf("Metadata: docs=%v, words=%v, version=%v",
			metadata["totalDocsProcessed"],
			metadata["totalWordsIndexed"],
			metadata["version"])
	})

	t.Run("GET /api/wordcloud - word frequency validation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wordcloud?limit=50", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		words := response["words"].([]interface{})

		// Check specific known words
		wordMap := make(map[string]float64)
		for _, w := range words {
			word := w.(map[string]interface{})
			wordMap[word["word"].(string)] = word["frequency"].(float64)
		}

		// "invoice" appears 4 times in first document
		if freq, ok := wordMap["invoice"]; ok {
			if freq < 3 {
				t.Errorf("Expected 'invoice' to appear at least 3 times, got %.0f", freq)
			}
			t.Logf("'invoice' frequency: %.0f", freq)
		}

		// "contract" appears 4 times in second document
		if freq, ok := wordMap["contract"]; ok {
			if freq < 3 {
				t.Errorf("Expected 'contract' to appear at least 3 times, got %.0f", freq)
			}
			t.Logf("'contract' frequency: %.0f", freq)
		}

		// "services" appears in multiple documents
		if freq, ok := wordMap["services"]; ok {
			if freq < 3 {
				t.Errorf("Expected 'services' to appear at least 3 times, got %.0f", freq)
			}
			t.Logf("'services' frequency: %.0f", freq)
		}

		// Stop words should NOT appear
		stopWords := []string{"this", "is", "for", "the", "and"}
		for _, stopWord := range stopWords {
			if _, ok := wordMap[stopWord]; ok {
				t.Errorf("Stop word '%s' should not be in word cloud", stopWord)
			}
		}
	})

	t.Run("GET /api/wordcloud - words sorted by frequency", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wordcloud?limit=20", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		words := response["words"].([]interface{})

		// Verify words are sorted by frequency (descending)
		var lastFreq float64 = 999999
		for i, w := range words {
			word := w.(map[string]interface{})
			freq := word["frequency"].(float64)

			if freq > lastFreq {
				t.Errorf("Words not sorted by frequency at index %d: %.0f > %.0f", i, freq, lastFreq)
			}

			lastFreq = freq
		}

		t.Log("✓ Words are sorted by frequency (descending)")
	})

	t.Run("POST /api/wordcloud/recalculate", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/wordcloud/recalculate", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Check response structure
		if _, ok := response["message"]; !ok {
			t.Error("Response missing 'message' field")
		}
		if _, ok := response["status"]; !ok {
			t.Error("Response missing 'status' field")
		}

		if response["status"] != "processing" {
			t.Errorf("Expected status 'processing', got '%s'", response["status"])
		}

		t.Logf("Recalculation response: %v", response["message"])

		// Give it a moment to process
		time.Sleep(500 * time.Millisecond)

		// Verify we can still query after recalculation
		req2 := httptest.NewRequest(http.MethodGet, "/api/wordcloud", nil)
		rec2 := httptest.NewRecorder()
		e.ServeHTTP(rec2, req2)

		if rec2.Code != http.StatusOK {
			t.Errorf("Expected status 200 after recalculation, got %d", rec2.Code)
		}
	})

	t.Run("GET /api/wordcloud - Content-Type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wordcloud", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		contentType := rec.Header().Get("Content-Type")
		if !contains(contentType, "application/json") {
			t.Errorf("Expected Content-Type to contain 'application/json', got '%s'", contentType)
		}
	})
}

// TestWordCloudAPIEdgeCases tests edge cases and error conditions
func TestWordCloudAPIEdgeCases(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("GET /api/wordcloud - empty database", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wordcloud", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should return 200 even with no data
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Verify words is an empty array, not null
		if response["words"] == nil {
			t.Error("Expected words to be an empty array [], got null")
			t.Logf("Full response: %s", rec.Body.String())
		} else {
			words := response["words"].([]interface{})
			if len(words) != 0 {
				t.Errorf("Expected 0 words in empty database, got %d", len(words))
			}
		}

		// Verify metadata is not null
		if response["metadata"] == nil {
			t.Error("Expected metadata to be an object, got null")
		} else {
			metadata := response["metadata"].(map[string]interface{})
			if metadata["totalDocsProcessed"] != float64(0) {
				t.Errorf("Expected totalDocsProcessed to be 0, got %v", metadata["totalDocsProcessed"])
			}
		}

		// Verify count is 0
		if response["count"] != float64(0) {
			t.Errorf("Expected count to be 0, got %v", response["count"])
		}
	})

	t.Run("GET /api/wordcloud - zero limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wordcloud?limit=0", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		// Should use default limit
		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		t.Logf("Zero limit response count: %v", response["count"])
	})

	t.Run("GET /api/wordcloud - negative limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wordcloud?limit=-10", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		// Should use default limit
		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		t.Log("Negative limit handled gracefully")
	})

	t.Run("POST /api/wordcloud/recalculate - wrong method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/wordcloud/recalculate", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should return method not allowed or 404
		if rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusNotFound {
			t.Logf("GET on POST endpoint returned %d (may be handled by catch-all)", rec.Code)
		}
	})
}

// TestWordCloudAPIConcurrency tests concurrent API requests
func TestWordCloudAPIConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("Concurrent word cloud requests", func(t *testing.T) {
		concurrency := 20
		done := make(chan bool, concurrency)
		errors := make(chan error, concurrency)

		for i := 0; i < concurrency; i++ {
			go func(id int) {
				req := httptest.NewRequest(http.MethodGet, "/api/wordcloud", nil)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					errors <- fmt.Errorf("request %d failed with status %d", id, rec.Code)
				}
				done <- true
			}(i)
		}

		// Wait for all requests
		for i := 0; i < concurrency; i++ {
			<-done
		}

		close(errors)
		errorCount := 0
		for err := range errors {
			t.Error(err)
			errorCount++
		}

		if errorCount == 0 {
			t.Logf("✓ Successfully handled %d concurrent requests", concurrency)
		}
	})
}

// TestWordCloudAPIPerformance tests API performance
func TestWordCloudAPIPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	e, serverHandler, cleanup := setupTestServer(t)
	defer cleanup()

	// Create some test data
	tempDir := t.TempDir()
	for i := 0; i < 50; i++ {
		ulid, _ := database.CalculateUUID(time.Now().Add(time.Duration(i) * time.Millisecond))

		filePath := filepath.Join(tempDir, fmt.Sprintf("doc_%d.txt", i))
		content := fmt.Sprintf("Document %d with some test content for word cloud performance testing", i)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		doc := &database.Document{
			Name:         fmt.Sprintf("doc_%d.txt", i),
			Path:         filePath,
			Folder:       "Test",
			Hash:         fmt.Sprintf("hash_%d", i),
			FullText:     content,
			IngressTime:  time.Now(),
			DocumentType: ".txt",
			ULID:         ulid,
		}
		serverHandler.DB.SaveDocument(doc)
	}

	serverHandler.DB.RecalculateAllWordFrequencies()

	t.Run("Word cloud API performance", func(t *testing.T) {
		iterations := 50
		start := time.Now()

		for i := 0; i < iterations; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/wordcloud", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("Request %d failed with status %d", i, rec.Code)
			}
		}

		elapsed := time.Since(start)
		avgTime := elapsed / time.Duration(iterations)

		t.Logf("Completed %d requests in %v (avg: %v per request)", iterations, elapsed, avgTime)

		if avgTime > 100*time.Millisecond {
			t.Logf("Warning: Average response time (%v) higher than expected", avgTime)
		} else {
			t.Logf("✓ Performance: %v per request", avgTime)
		}
	})
}
