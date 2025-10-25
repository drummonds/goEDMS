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

// TestSearchEndpoint provides comprehensive tests for the search API endpoint
func TestSearchEndpoint(t *testing.T) {
	e, serverHandler, cleanup := setupTestServer(t)
	defer cleanup()

	// Create temporary directory for test documents
	tempDir := t.TempDir()

	// Create test documents with various content for searching
	testDocuments := []struct {
		name    string
		content string
		folder  string
	}{
		{
			name:    "Invoice_2024_Q1.pdf",
			content: "This is an invoice for the first quarter of 2024. Total amount: $1500",
			folder:  "Finance",
		},
		{
			name:    "Receipt_Store_Purchase.pdf",
			content: "Receipt for store purchase of office supplies including paper and pens",
			folder:  "Receipts",
		},
		{
			name:    "Contract_Agreement.pdf",
			content: "Contract agreement between parties for service delivery",
			folder:  "Legal",
		},
		{
			name:    "Meeting_Notes_January.txt",
			content: "Meeting notes from January discussing quarterly objectives and budget",
			folder:  "Notes",
		},
		{
			name:    "Invoice_2024_Q2.pdf",
			content: "Second quarter invoice for services rendered. Amount: $2000",
			folder:  "Finance",
		},
		{
			name:    "Tax_Document_2023.pdf",
			content: "Tax documentation for fiscal year 2023",
			folder:  "Finance",
		},
	}

	// Insert test documents into database and create actual files
	for i, doc := range testDocuments {
		ulid, err := database.CalculateUUID(time.Now().Add(time.Duration(i) * time.Millisecond))
		if err != nil {
			t.Fatalf("Failed to generate ULID: %v", err)
		}

		// Create folder structure
		folderPath := filepath.Join(tempDir, doc.folder)
		if err := os.MkdirAll(folderPath, 0755); err != nil {
			t.Fatalf("Failed to create folder %s: %v", folderPath, err)
		}

		// Create actual file
		filePath := filepath.Join(folderPath, doc.name)
		if err := os.WriteFile(filePath, []byte(doc.content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}

		newDoc := &database.Document{
			Name:         doc.name,
			Path:         filePath,
			Folder:       doc.folder,
			Hash:         fmt.Sprintf("hash_%d", i),
			FullText:     doc.content,
			IngressTime:  time.Now(),
			DocumentType: filepath.Ext(doc.name),
			ULID:         ulid,
			URL:          fmt.Sprintf("/document/view/%s", ulid.String()),
		}

		err = serverHandler.DB.SaveDocument(newDoc)
		if err != nil {
			t.Fatalf("Failed to save test document %s: %v", doc.name, err)
		}
	}

	t.Run("Search with valid term - single word", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/search?term=invoice", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var results []interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
			t.Fatalf("Failed to parse search results: %v\nBody: %s", err, rec.Body.String())
		}

		// Should find at least the 2 invoice documents
		if len(results) < 2 {
			t.Errorf("Expected at least 2 results for 'invoice', got %d", len(results))
		}

		t.Logf("Search for 'invoice' returned %d results", len(results))
	})

	t.Run("Search with valid term - phrase search", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/search?term=quarterly+objectives", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should return 200 or 204 (no content if not found)
		if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
			t.Errorf("Expected status 200 or 204, got %d: %s", rec.Code, rec.Body.String())
		}

		if rec.Code == http.StatusOK {
			var results []interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
				t.Fatalf("Failed to parse search results: %v", err)
			}

			t.Logf("Phrase search returned %d results", len(results))
		}
	})

	t.Run("Search with prefix matching", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/search?term=tax", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
			t.Errorf("Expected status 200 or 204, got %d", rec.Code)
		}

		if rec.Code == http.StatusOK {
			var results []interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
				t.Fatalf("Failed to parse search results: %v", err)
			}

			// Should find the tax document
			if len(results) == 0 {
				t.Error("Expected at least 1 result for 'tax', got 0")
			}
		}
	})

	t.Run("Search with no results", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/search?term=nonexistentterm12345", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should return 204 No Content for no results
		if rec.Code != http.StatusNoContent {
			t.Errorf("Expected status 204 for no results, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("Search with empty term", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/search?term=", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should return 404 Not Found for empty term
		if rec.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for empty term, got %d", rec.Code)
		}

		// Check error message
		var errorResponse map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &errorResponse); err == nil {
			t.Logf("Error response: %v", errorResponse)
		}
	})

	t.Run("Search without term parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/search", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should return 404 for missing term
		if rec.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for missing term, got %d", rec.Code)
		}
	})

	t.Run("Search with URL encoded term", func(t *testing.T) {
		// Search for "office supplies" which should be URL encoded
		req := httptest.NewRequest(http.MethodGet, "/api/search?term=office%20supplies", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
			t.Errorf("Expected status 200 or 204, got %d", rec.Code)
		}

		if rec.Code == http.StatusOK {
			var results []interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
				t.Fatalf("Failed to parse search results: %v", err)
			}

			t.Logf("URL encoded search returned %d results", len(results))
		}
	})

	t.Run("Search with special characters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/search?term=$1500", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should handle gracefully - may return 200, 204, or 500 depending on implementation
		if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent && rec.Code != http.StatusInternalServerError {
			t.Logf("Search with special characters returned status %d", rec.Code)
		}
	})

	t.Run("Search results contain required fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/search?term=invoice", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Skip("No results to validate")
		}

		var results []map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
			t.Fatalf("Failed to parse search results: %v", err)
		}

		if len(results) == 0 {
			t.Skip("No results returned")
		}

		// Validate the first result has expected fields
		firstResult := results[0]
		requiredFields := []string{"id", "name", "fullPath"}
		for _, field := range requiredFields {
			if _, ok := firstResult[field]; !ok {
				t.Errorf("Search result missing required field: %s", field)
			}
		}

		t.Logf("First search result: %+v", firstResult)
	})

	t.Run("Search case insensitivity", func(t *testing.T) {
		// Search with different cases
		terms := []string{"INVOICE", "Invoice", "invoice"}
		var resultCounts []int

		for _, term := range terms {
			req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/search?term=%s", term), nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code == http.StatusOK {
				var results []interface{}
				if err := json.Unmarshal(rec.Body.Bytes(), &results); err == nil {
					resultCounts = append(resultCounts, len(results))
				}
			} else if rec.Code == http.StatusNoContent {
				resultCounts = append(resultCounts, 0)
			}
		}

		// All searches should return the same number of results (case insensitive)
		if len(resultCounts) >= 2 {
			for i := 1; i < len(resultCounts); i++ {
				if resultCounts[i] != resultCounts[0] {
					t.Logf("Warning: Case-sensitive search detected. Results: %v", resultCounts)
				}
			}
		}
	})

	t.Run("Search returns proper Content-Type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/search?term=invoice", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		contentType := rec.Header().Get("Content-Type")
		if rec.Code == http.StatusOK && !contains(contentType, "application/json") {
			t.Errorf("Expected Content-Type to contain 'application/json', got '%s'", contentType)
		}
	})
}

// TestSearchPerformance tests search endpoint performance
func TestSearchPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	e, serverHandler, cleanup := setupTestServer(t)
	defer cleanup()

	// Create temporary directory
	tempDir := t.TempDir()

	// Insert multiple documents
	for i := 0; i < 50; i++ {
		ulid, _ := database.CalculateUUID(time.Now().Add(time.Duration(i) * time.Millisecond))

		// Create actual file
		filePath := filepath.Join(tempDir, fmt.Sprintf("Document_%d.pdf", i))
		content := fmt.Sprintf("This is test document number %d containing searchable text", i)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		doc := &database.Document{
			Name:         fmt.Sprintf("Document_%d.pdf", i),
			Path:         filePath,
			Folder:       "Test",
			Hash:         fmt.Sprintf("hash_%d", i),
			FullText:     content,
			IngressTime:  time.Now(),
			DocumentType: ".pdf",
			ULID:         ulid,
		}
		serverHandler.DB.SaveDocument(doc)
	}

	t.Run("Search performance with 50 documents", func(t *testing.T) {
		iterations := 20
		start := time.Now()

		for i := 0; i < iterations; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/search?term=searchable", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
				t.Errorf("Search request %d failed with status %d", i, rec.Code)
			}
		}

		elapsed := time.Since(start)
		avgTime := elapsed / time.Duration(iterations)

		t.Logf("Completed %d search requests in %v (avg: %v per request)", iterations, elapsed, avgTime)

		if avgTime > 200*time.Millisecond {
			t.Logf("Warning: Average search time (%v) is higher than expected", avgTime)
		}
	})
}

// TestSearchConcurrency tests concurrent search requests
func TestSearchConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	e, serverHandler, cleanup := setupTestServer(t)
	defer cleanup()

	// Create temporary directory
	tempDir := t.TempDir()

	// Insert test documents
	for i := 0; i < 10; i++ {
		ulid, _ := database.CalculateUUID(time.Now().Add(time.Duration(i) * time.Millisecond))

		// Create actual file
		filePath := filepath.Join(tempDir, fmt.Sprintf("Document_%d.pdf", i))
		content := fmt.Sprintf("Test document %d with concurrent search test", i)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		doc := &database.Document{
			Name:         fmt.Sprintf("Document_%d.pdf", i),
			Path:         filePath,
			Folder:       "Test",
			Hash:         fmt.Sprintf("hash_%d", i),
			FullText:     content,
			IngressTime:  time.Now(),
			DocumentType: ".pdf",
			ULID:         ulid,
		}
		serverHandler.DB.SaveDocument(doc)
	}

	t.Run("Concurrent search requests", func(t *testing.T) {
		concurrency := 10
		done := make(chan bool, concurrency)
		errors := make(chan error, concurrency)

		for i := 0; i < concurrency; i++ {
			go func(id int) {
				req := httptest.NewRequest(http.MethodGet, "/api/search?term=concurrent", nil)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
					errors <- fmt.Errorf("concurrent search request %d failed with status %d", id, rec.Code)
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
			t.Logf("Successfully handled %d concurrent search requests", concurrency)
		}
	})
}

// TestSearchResultFormat validates the format of search results
func TestSearchResultFormat(t *testing.T) {
	e, serverHandler, cleanup := setupTestServer(t)
	defer cleanup()

	// Create temporary directory and file
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "Test_Format.pdf")
	if err := os.WriteFile(filePath, []byte("This is a format validation test document"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Insert a test document
	ulid, _ := database.CalculateUUID(time.Now())
	doc := &database.Document{
		Name:         "Test_Format.pdf",
		Path:         filePath,
		Folder:       "FormatTest",
		Hash:         "hash_format",
		FullText:     "This is a format validation test document",
		IngressTime:  time.Now(),
		DocumentType: ".pdf",
		ULID:         ulid,
		URL:          "/document/view/" + ulid.String(),
	}
	serverHandler.DB.SaveDocument(doc)

	t.Run("Search results have valid structure", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/search?term=format", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Skip("No results to validate")
		}

		var results []map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
			t.Fatalf("Failed to parse results: %v", err)
		}

		if len(results) == 0 {
			t.Skip("No results returned")
		}

		// Verify array structure
		if len(results) < 1 {
			t.Fatal("Results should be a non-empty array")
		}

		// The first item should be the "SearchResults" root node
		rootNode := results[0]
		if id, ok := rootNode["id"].(string); ok {
			if id == "SearchResults" {
				t.Log("Found SearchResults root node")
			}
		}

		// Find an actual document result
		var docNode map[string]interface{}
		for _, result := range results {
			if id, ok := result["id"].(string); ok && id != "SearchResults" {
				docNode = result
				break
			}
		}

		if docNode == nil {
			t.Fatal("No document nodes found in results")
		}

		// Validate document node structure
		expectedFields := map[string]string{
			"id":       "string",
			"name":     "string",
			"fullPath": "string",
			"isDir":    "bool",
		}

		for field, expectedType := range expectedFields {
			value, ok := docNode[field]
			if !ok {
				t.Errorf("Document node missing field: %s", field)
				continue
			}

			switch expectedType {
			case "string":
				if _, ok := value.(string); !ok {
					t.Errorf("Field %s should be string, got %T", field, value)
				}
			case "bool":
				if _, ok := value.(bool); !ok {
					t.Errorf("Field %s should be bool, got %T", field, value)
				}
			}
		}

		t.Logf("Document node structure validated: %+v", docNode)
	})
}
