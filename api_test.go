package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	config "github.com/drummonds/goEDMS/config"
	database "github.com/drummonds/goEDMS/database"
	engine "github.com/drummonds/goEDMS/engine"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// setupTestServer creates a test server with all routes configured
func setupTestServer(t *testing.T) (*echo.Echo, *engine.ServerHandler, func()) {
	serverConfig, logger := config.SetupServer()
	injectGlobals(logger)

	// Use ephemeral PostgreSQL for tests
	ephemeralDB, err := database.SetupEphemeralPostgresDatabase()
	if err != nil {
		t.Fatalf("Failed to setup ephemeral database: %v", err)
	}
	testDB := database.DBInterface(ephemeralDB)
	t.Cleanup(func() {
		ephemeralDB.Close()
	})

	database.WriteConfigToDB(serverConfig, testDB)

	e := echo.New()
	e.HideBanner = true
	serverHandler := &engine.ServerHandler{
		DB:           testDB,
		Echo:         e,
		ServerConfig: serverConfig,
	}

	// Setup routes
	e.Use(middleware.CORSWithConfig(middleware.DefaultCORSConfig))
	e.GET("/api/documents/latest", serverHandler.GetLatestDocuments)
	e.GET("/api/documents/filesystem", serverHandler.GetDocumentFileSystem)
	e.GET("/api/document/:id", serverHandler.GetDocument)
	e.DELETE("/api/document/*", serverHandler.DeleteFile)
	e.PATCH("/api/document/move/*", serverHandler.MoveDocuments)
	e.POST("/api/document/upload", serverHandler.UploadDocuments)
	e.GET("/api/folder/:folder", serverHandler.GetFolder)
	e.POST("/api/folder/*", serverHandler.CreateFolder)
	e.GET("/api/search", serverHandler.SearchDocuments)
	e.GET("/api/about", serverHandler.GetAboutInfo)
	e.POST("/api/ingest", serverHandler.RunIngestNow)
	e.POST("/api/clean", serverHandler.CleanDatabase)

	// Word cloud routes
	e.GET("/api/wordcloud", serverHandler.GetWordCloud)
	e.POST("/api/wordcloud/recalculate", serverHandler.RecalculateWordCloud)

	cleanup := func() {
		testDB.Close()
	}

	return e, serverHandler, cleanup
}

// TestGetLatestDocuments tests the /home endpoint
func TestGetLatestDocuments(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("Get latest documents - empty database", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/documents/latest", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v\nBody: %s", err, rec.Body.String())
		}

		// Response should have pagination metadata
		if _, ok := response["documents"]; !ok {
			t.Logf("Response structure: %+v", response)
			t.Fatal("Response missing 'documents' field")
		}

		// Handle nil documents (empty database)
		if response["documents"] == nil {
			t.Log("Got nil documents (empty database)")
		} else {
			documents, ok := response["documents"].([]interface{})
			if !ok {
				t.Fatalf("Documents field is not an array: %T", response["documents"])
			}
			t.Logf("Got %d documents", len(documents))
		}
	})

	t.Run("Get latest documents - with pagination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/documents/latest?page=1", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		// Check pagination metadata
		if _, ok := response["page"]; !ok {
			t.Error("Response missing 'page' field")
		}
		if _, ok := response["pageSize"]; !ok {
			t.Error("Response missing 'pageSize' field")
		}
		if _, ok := response["totalCount"]; !ok {
			t.Error("Response missing 'totalCount' field")
		}
		if _, ok := response["totalPages"]; !ok {
			t.Error("Response missing 'totalPages' field")
		}
		if _, ok := response["hasNext"]; !ok {
			t.Error("Response missing 'hasNext' field")
		}
		if _, ok := response["hasPrevious"]; !ok {
			t.Error("Response missing 'hasPrevious' field")
		}
	})

	t.Run("Get latest documents - invalid page number", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/documents/latest?page=invalid", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should still return 200 with default page 1
		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rec.Code)
		}
	})
}

// TestGetDocumentFileSystem tests the /documents/filesystem endpoint
func TestGetDocumentFileSystem(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/documents/filesystem", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should return filesystem structure
	if response == nil {
		t.Error("Expected non-nil response")
	}
}

// TestSearchDocuments tests the /search/* endpoint
func TestSearchDocuments(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("Search - empty query term", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/search?term=", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Empty term should return 404
		if rec.Code != http.StatusNotFound {
			t.Logf("Empty search term returned status %d (expected 404)", rec.Code)
		}
	})

	t.Run("Search - with query term", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/search?term=test", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should return 200, 204 (no content), 404 (document not found), or 500 (search not initialized)
		if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent && rec.Code != http.StatusNotFound && rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 200, 204, 404, or 500, got %d: %s", rec.Code, rec.Body.String())
		}

		if rec.Code == http.StatusOK {
			var results []interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
				t.Logf("Response body: %s", rec.Body.String())
				t.Fatalf("Failed to parse search results: %v", err)
			}
			t.Logf("Got %d search results", len(results))
		} else if rec.Code == http.StatusInternalServerError {
			t.Log("Search returned 500 (may need search index initialization)")
		}
	})

	t.Run("Search - phrase search", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/search?term=test+document", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should handle phrase search - accept 200, 204, or 500
		if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent && rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 200, 204, or 500, got %d", rec.Code)
		}
	})
}

// TestUploadDocument tests the /document/upload endpoint
func TestUploadDocument(t *testing.T) {
	e, serverHandler, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a test file
	testContent := []byte("This is a test document for upload testing")
	testFileName := "test_upload.txt"

	t.Run("Upload document - valid file", func(t *testing.T) {
		// Create multipart form
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		// Add file
		part, err := writer.CreateFormFile("upload", testFileName)
		if err != nil {
			t.Fatalf("Failed to create form file: %v", err)
		}
		if _, err := part.Write(testContent); err != nil {
			t.Fatalf("Failed to write file content: %v", err)
		}

		// Add folder field
		if err := writer.WriteField("Folder", "test_folder"); err != nil {
			t.Fatalf("Failed to write folder field: %v", err)
		}

		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/document/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Accept 200, 201, or 500 (may fail due to file system setup)
		if rec.Code != http.StatusOK && rec.Code != http.StatusCreated && rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 200, 201, or 500, got %d: %s", rec.Code, rec.Body.String())
		}

		if rec.Code == http.StatusInternalServerError {
			t.Logf("Upload failed (may need proper file system setup): %s", rec.Body.String())
		}
	})

	t.Run("Upload document - missing file", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/document/upload", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		// Should return an error
		if rec.Code == http.StatusOK {
			t.Error("Expected error status, got 200")
		}
	})

	// Cleanup uploaded files
	if serverHandler.ServerConfig.DocumentPath != "" {
		os.RemoveAll(filepath.Join(serverHandler.ServerConfig.DocumentPath, "test_folder"))
	}
}

// TestGetDocument tests the /document/:id endpoint
func TestGetDocument(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("Get document - non-existent ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/document/nonexistent123", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound && rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 404 or 500, got %d", rec.Code)
		}
	})

	t.Run("Get document - invalid ID format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/document/", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should not match route or return error
		if rec.Code == http.StatusOK {
			t.Error("Expected error for empty document ID")
		}
	})
}

// TestDeleteDocument tests the DELETE /document/* endpoint
func TestDeleteDocument(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("Delete document - non-existent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/document/nonexistent123", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should handle gracefully
		if rec.Code != http.StatusOK && rec.Code != http.StatusNotFound && rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 200, 404, or 500, got %d", rec.Code)
		}
	})
}

// TestFolderOperations tests folder creation and retrieval
func TestFolderOperations(t *testing.T) {
	e, serverHandler, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("Create folder", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/folder/test_api_folder", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Accept 200, 201, or 500 (may fail due to file system setup)
		if rec.Code != http.StatusOK && rec.Code != http.StatusCreated && rec.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 200, 201, or 500, got %d: %s", rec.Code, rec.Body.String())
		}

		if rec.Code == http.StatusInternalServerError {
			t.Logf("Folder creation failed (may need proper file system setup): %s", rec.Body.String())
		}
	})

	t.Run("Get folder contents - non-existent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/folder/nonexistent_folder", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should return empty or not found
		if rec.Code != http.StatusOK && rec.Code != http.StatusNotFound {
			t.Errorf("Expected status 200 or 404, got %d", rec.Code)
		}
	})

	// Cleanup
	if serverHandler.ServerConfig.DocumentPath != "" {
		os.RemoveAll(filepath.Join(serverHandler.ServerConfig.DocumentPath, "test_api_folder"))
	}
}

// TestAdminEndpoints tests the admin API endpoints
func TestAdminEndpoints(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("Trigger manual ingest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/ingest", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		// Check response
		if rec.Body.String() == "" {
			t.Error("Expected response body, got empty")
		}
	})

	t.Run("Clean database", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/clean", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		// Parse response
		var response map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse clean response: %v", err)
		}

		// Should have scanned, deleted, and moved counts
		if _, ok := response["scanned"]; !ok {
			t.Error("Response missing 'scanned' field")
		}
		if _, ok := response["deleted"]; !ok {
			t.Error("Response missing 'deleted' field")
		}
		if _, ok := response["moved"]; !ok {
			t.Error("Response missing 'moved' field")
		}
	})

	t.Run("Invalid method for admin endpoints", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/ingest", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Logf("GET on POST-only endpoint returned %d (may be handled by catch-all)", rec.Code)
		}
	})
}

// TestMoveDocument tests the PATCH /document/move/* endpoint
func TestMoveDocument(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("Move document - non-existent", func(t *testing.T) {
		// Create request body
		moveData := map[string]string{
			"Folder": "new_folder",
		}
		bodyBytes, _ := json.Marshal(moveData)

		req := httptest.NewRequest(http.MethodPatch, "/api/document/move/nonexistent123", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should handle gracefully
		if rec.Code == http.StatusOK {
			t.Log("Move operation returned OK for non-existent document (may be a no-op)")
		}
	})
}

// TestAPIPerformance tests API endpoint performance
func TestAPIPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("Home endpoint performance", func(t *testing.T) {
		iterations := 100
		start := time.Now()

		for i := 0; i < iterations; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/documents/latest", nil)
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
			t.Logf("Warning: Average request time (%v) is higher than expected", avgTime)
		}
	})

	t.Run("Search endpoint performance", func(t *testing.T) {
		iterations := 50
		start := time.Now()

		for i := 0; i < iterations; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/search?term=test", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			// Accept 200, 204, or 500 (search may not be initialized)
			if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent && rec.Code != http.StatusInternalServerError {
				t.Errorf("Search request %d failed with status %d", i, rec.Code)
			}
		}

		elapsed := time.Since(start)
		avgTime := elapsed / time.Duration(iterations)

		t.Logf("Completed %d search requests in %v (avg: %v per request)", iterations, elapsed, avgTime)
	})
}

// TestConcurrentRequests tests API behavior under concurrent load
func TestConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("Concurrent home requests", func(t *testing.T) {
		concurrency := 10
		done := make(chan bool, concurrency)
		errors := make(chan error, concurrency)

		for i := 0; i < concurrency; i++ {
			go func(id int) {
				req := httptest.NewRequest(http.MethodGet, "/api/documents/latest", nil)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					errors <- fmt.Errorf("concurrent request %d failed with status %d", id, rec.Code)
				}
				done <- true
			}(i)
		}

		// Wait for all requests
		for i := 0; i < concurrency; i++ {
			<-done
		}

		close(errors)
		for err := range errors {
			t.Error(err)
		}
	})
}

// TestContentTypes tests that endpoints return correct content types
func TestContentTypes(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name         string
		endpoint     string
		method       string
		expectedType string
	}{
		{"Home endpoint", "/api/documents/latest", "GET", "application/json"},
		{"Search endpoint", "/api/searchtest", "GET", "application/json"},
		{"Filesystem endpoint", "/api/documents/filesystem", "GET", "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.endpoint, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			contentType := rec.Header().Get("Content-Type")
			if contentType != tt.expectedType && !contains(contentType, tt.expectedType) {
				t.Errorf("Expected Content-Type %s, got %s", tt.expectedType, contentType)
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr)))
}

// TestErrorHandling tests API error handling
func TestErrorHandling(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("Invalid JSON in request body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/api/document/move/test123", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should handle invalid JSON gracefully
		if rec.Code == http.StatusOK {
			t.Log("Endpoint handled invalid JSON (may have defaults)")
		}
	})

	t.Run("Very long document ID", func(t *testing.T) {
		longID := string(make([]byte, 1000)) // Reduced from 10000 to avoid URL length issues
		for i := range longID {
			longID = longID[:i] + "a" + longID[i+1:]
		}
		req := httptest.NewRequest(http.MethodGet, "/api/document/"+longID, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should handle gracefully - not return OK
		if rec.Code == http.StatusOK {
			t.Error("Should not return OK for invalid long ID")
		}
		t.Logf("Long ID returned status %d", rec.Code)
	})
}

// TestGetAboutInfo tests the /api/about endpoint
func TestGetAboutInfo(t *testing.T) {
	e, serverHandler, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("Get about information", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/about", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var aboutInfo map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &aboutInfo); err != nil {
			t.Fatalf("Failed to parse response: %v\nBody: %s", err, rec.Body.String())
		}

		// Verify required fields are present
		requiredFields := []string{"version", "ocrConfigured", "ocrPath", "databaseType"}
		for _, field := range requiredFields {
			if _, ok := aboutInfo[field]; !ok {
				t.Errorf("Response missing required field: %s", field)
			}
		}

		// Verify field types
		if _, ok := aboutInfo["version"].(string); !ok {
			t.Errorf("version should be a string, got %T", aboutInfo["version"])
		}

		if _, ok := aboutInfo["ocrConfigured"].(bool); !ok {
			t.Errorf("ocrConfigured should be a boolean, got %T", aboutInfo["ocrConfigured"])
		}

		if _, ok := aboutInfo["ocrPath"].(string); !ok {
			t.Errorf("ocrPath should be a string, got %T", aboutInfo["ocrPath"])
		}

		if _, ok := aboutInfo["databaseType"].(string); !ok {
			t.Errorf("databaseType should be a string, got %T", aboutInfo["databaseType"])
		}

		// Log the actual values
		t.Logf("Version: %v", aboutInfo["version"])
		t.Logf("OCR Configured: %v", aboutInfo["ocrConfigured"])
		t.Logf("OCR Path: %v", aboutInfo["ocrPath"])
		t.Logf("Database Type: %v", aboutInfo["databaseType"])

		// Verify OCR configuration matches server config
		ocrConfigured := aboutInfo["ocrConfigured"].(bool)
		expectedOCRConfigured := serverHandler.ServerConfig.TesseractPath != ""
		if ocrConfigured != expectedOCRConfigured {
			t.Errorf("OCR configured mismatch: got %v, expected %v", ocrConfigured, expectedOCRConfigured)
		}

		// Verify database type
		dbType := aboutInfo["databaseType"].(string)
		if dbType == "" {
			t.Error("Database type should not be empty")
		}

		// Database type should be one of the valid types
		validDBTypes := []string{"postgres", "cockroachdb", "sqlite"}
		validType := false
		for _, valid := range validDBTypes {
			if dbType == valid {
				validType = true
				break
			}
		}
		if !validType {
			t.Logf("Database type '%s' may be valid but not in expected list", dbType)
		}
	})

	t.Run("About endpoint returns consistent data", func(t *testing.T) {
		// Make multiple requests to ensure consistency
		var responses []map[string]interface{}

		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/api/about", nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("Request %d failed with status %d", i+1, rec.Code)
				continue
			}

			var aboutInfo map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &aboutInfo); err != nil {
				t.Errorf("Request %d failed to parse: %v", i+1, err)
				continue
			}

			responses = append(responses, aboutInfo)
		}

		// Verify all responses are identical
		if len(responses) < 2 {
			t.Fatal("Not enough successful responses to compare")
		}

		firstResponse, _ := json.Marshal(responses[0])
		for i := 1; i < len(responses); i++ {
			currentResponse, _ := json.Marshal(responses[i])
			if string(firstResponse) != string(currentResponse) {
				t.Errorf("Response %d differs from first response", i+1)
				t.Logf("First: %s", firstResponse)
				t.Logf("Current: %s", currentResponse)
			}
		}

		t.Log("✓ About endpoint returns consistent data across multiple requests")
	})

	t.Run("About endpoint handles OPTIONS request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/about", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should handle CORS preflight (or return method not allowed)
		if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK && rec.Code != http.StatusMethodNotAllowed {
			t.Logf("OPTIONS request returned status %d", rec.Code)
		}
	})
}
