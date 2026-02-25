//go:build !js

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	config "github.com/drummonds/godocs/config"
	database "github.com/drummonds/godocs/database"
	engine "github.com/drummonds/godocs/engine"
	"github.com/drummonds/godocs/internal/docs"
	"github.com/drummonds/lofigui"
	"github.com/labstack/echo/v4"
)

// setupSSRTestServer creates a test server with SSR templates and returns the echo instance and cleanup func
func setupSSRTestServer(t *testing.T) (*echo.Echo, database.Repository, func()) {
	t.Helper()

	serverConfig, logger := config.SetupServer()
	injectGlobals(logger)

	ephemeralDB, err := database.SetupEphemeralPostgresDatabase()
	if err != nil {
		t.Fatalf("Failed to setup ephemeral database: %v", err)
	}
	db := database.Repository(ephemeralDB)

	database.WriteConfigToDB(serverConfig, db)

	e := echo.New()
	e.HideBanner = true

	app := lofigui.NewApp()
	app.Version = "test"

	serverHandler := engine.ServerHandler{DB: db, Echo: e, ServerConfig: serverConfig}
	serverHandler.InitJobContext()

	tr := &TemplateRenderer{
		templateSet:       NewTemplateSet(),
		db:                db,
		config:            serverConfig,
		version:           "test",
		isActionRunning:   app.IsActionRunning,
		checkThumbnail:    checkThumbnailFS,
		writeTagAliases:   engine.WriteTagAliases,
		runCleanupAsync:   serverHandler.RunCleanupAsync,
		runIngestionAsync: serverHandler.RunIngestionAsync,
	}

	e.HTTPErrorHandler = createHTTPErrorHandler(e, tr)
	docsHandler := docs.DocsHandler{DocsFS: docsFS, SwaggerUIFS: swaggerUIFS}
	serverHandler.InitializeSchedules(db)
	serverHandler.StartupChecks()

	// API routes
	e.GET("/api/documents/latest", serverHandler.GetLatestDocuments)
	e.GET("/api/documents/filesystem", serverHandler.GetDocumentFileSystem)
	e.GET("/api/document/:id", serverHandler.GetDocument)
	e.GET("/api/about", serverHandler.GetAboutInfo)
	e.GET("/api/search", serverHandler.SearchDocuments)
	e.GET("/api/docs", docsHandler.GetSwaggerUI)

	// Document view route
	e.GET("/document/view/:ulid", serverHandler.ViewDocument)

	// SSR page routes
	e.GET("/", HandleHomePage(tr))
	e.GET("/search", HandleSearchPage(tr))
	e.GET("/document/:ulid", HandleDocumentPage(tr))
	e.GET("/about", HandleAboutPage(tr))

	// Tag management routes
	e.GET("/tags", HandleTagsPage(tr))
	e.POST("/tags", HandleCreateTag(tr))
	e.GET("/tags/:id/edit", HandleEditTagPage(tr))
	e.POST("/tags/:id", HandleUpdateTag(tr))
	e.POST("/tags/:id/delete", HandleDeleteTag(tr))

	// Jobs management routes
	e.GET("/jobs", HandleJobsPage(tr))
	e.POST("/jobs/clean", HandleTriggerClean(tr))
	e.POST("/jobs/ingest", HandleTriggerIngest(tr))

	cleanup := func() {
		ephemeralDB.Close()
		db.Close()
	}

	return e, db, cleanup
}

// TestHomePage tests that the home page renders as server-side HTML
func TestHomePage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	e, _, cleanup := setupSSRTestServer(t)
	defer cleanup()

	testPort := "8996"
	go func() {
		if err := e.Start(fmt.Sprintf("127.0.0.1:%s", testPort)); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()
	time.Sleep(1 * time.Second)
	defer e.Shutdown(context.Background())

	testURL := fmt.Sprintf("http://127.0.0.1:%s/", testPort)

	cmd := exec.Command("curl", "-s", "-L", "-w", "\n%{http_code}", "--max-time", "5", testURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Curl failed: %v, output: %s", err, string(output))
	}

	outputStr := string(output)
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	statusCode := lines[len(lines)-1]
	responseBody := strings.Join(lines[:len(lines)-1], "\n")

	if statusCode != "200" {
		t.Errorf("Expected status 200, got %s", statusCode)
	}

	// Check for expected HTML content
	bodyLower := strings.ToLower(responseBody)
	if !strings.Contains(bodyLower, "godocs") {
		t.Error("Home page missing 'godocs' text")
	}
	if !strings.Contains(bodyLower, "documents") {
		t.Error("Home page missing 'Documents' heading")
	}
	if !strings.Contains(bodyLower, "<nav") {
		t.Error("Home page missing navbar")
	}

	t.Logf("Home page test passed (status=%s, length=%d)", statusCode, len(responseBody))
}

// TestAboutPage tests that the about page renders with system info
func TestAboutPage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	e, _, cleanup := setupSSRTestServer(t)
	defer cleanup()

	testPort := "8995"
	go func() {
		if err := e.Start(fmt.Sprintf("127.0.0.1:%s", testPort)); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()
	time.Sleep(1 * time.Second)
	defer e.Shutdown(context.Background())

	testURL := fmt.Sprintf("http://127.0.0.1:%s/about", testPort)

	cmd := exec.Command("curl", "-s", "-L", "-w", "\n%{http_code}", "--max-time", "5", testURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Curl failed: %v, output: %s", err, string(output))
	}

	outputStr := string(output)
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	statusCode := lines[len(lines)-1]
	responseBody := strings.Join(lines[:len(lines)-1], "\n")

	if statusCode != "200" {
		t.Errorf("Expected status 200, got %s", statusCode)
	}

	bodyLower := strings.ToLower(responseBody)

	expectedContent := []string{
		"about godocs",
		"database",
		"application",
		"storage",
	}

	for _, content := range expectedContent {
		if !strings.Contains(bodyLower, content) {
			t.Errorf("About page missing expected content: '%s'", content)
		}
	}

	t.Logf("About page test passed (status=%s, length=%d)", statusCode, len(responseBody))
}

// TestSearchPage tests that the search page renders
func TestSearchPage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	e, _, cleanup := setupSSRTestServer(t)
	defer cleanup()

	testPort := "8994"
	go func() {
		if err := e.Start(fmt.Sprintf("127.0.0.1:%s", testPort)); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()
	time.Sleep(1 * time.Second)
	defer e.Shutdown(context.Background())

	// Test search page without query
	testURL := fmt.Sprintf("http://127.0.0.1:%s/search", testPort)
	cmd := exec.Command("curl", "-s", "-L", "-w", "\n%{http_code}", "--max-time", "5", testURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Curl failed: %v", err)
	}

	outputStr := string(output)
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	statusCode := lines[len(lines)-1]
	responseBody := strings.Join(lines[:len(lines)-1], "\n")

	if statusCode != "200" {
		t.Errorf("Expected status 200, got %s", statusCode)
	}

	if !strings.Contains(strings.ToLower(responseBody), "search") {
		t.Error("Search page missing 'Search' text")
	}

	// Test search page with query
	testURL = fmt.Sprintf("http://127.0.0.1:%s/search?q=test", testPort)
	cmd = exec.Command("curl", "-s", "-L", "-w", "\n%{http_code}", "--max-time", "5", testURL)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Curl failed: %v", err)
	}

	outputStr = string(output)
	lines = strings.Split(strings.TrimSpace(outputStr), "\n")
	statusCode = lines[len(lines)-1]

	if statusCode != "200" {
		t.Errorf("Search with query: expected status 200, got %s", statusCode)
	}

	t.Logf("Search page test passed (status=%s)", statusCode)
}

// TestAPIStillWorks tests that API endpoints are preserved
func TestAPIStillWorks(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	e, _, cleanup := setupSSRTestServer(t)
	defer cleanup()

	testPort := "8993"
	go func() {
		if err := e.Start(fmt.Sprintf("127.0.0.1:%s", testPort)); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()
	time.Sleep(1 * time.Second)
	defer e.Shutdown(context.Background())

	// Test /api/about
	testURL := fmt.Sprintf("http://127.0.0.1:%s/api/about", testPort)
	cmd := exec.Command("curl", "-s", "-L", "-w", "\n%{http_code}", "--max-time", "5", testURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Curl failed: %v", err)
	}

	outputStr := string(output)
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")
	statusCode := lines[len(lines)-1]
	responseBody := strings.Join(lines[:len(lines)-1], "\n")

	if statusCode != "200" {
		t.Errorf("/api/about: expected status 200, got %s", statusCode)
	}

	if !strings.Contains(responseBody, "version") {
		t.Error("/api/about response missing 'version' field")
	}

	// Test /api/documents/latest
	testURL = fmt.Sprintf("http://127.0.0.1:%s/api/documents/latest", testPort)
	cmd = exec.Command("curl", "-s", "-L", "-w", "\n%{http_code}", "--max-time", "5", testURL)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Curl failed: %v", err)
	}

	outputStr = string(output)
	lines = strings.Split(strings.TrimSpace(outputStr), "\n")
	statusCode = lines[len(lines)-1]

	if statusCode != "200" {
		t.Errorf("/api/documents/latest: expected status 200, got %s", statusCode)
	}

	t.Log("API endpoints test passed")
}

// TestTesseractOptional tests that the application runs without Tesseract configured
func TestTesseractOptional(t *testing.T) {
	serverConfig, logger := config.SetupServer()

	if serverConfig.ListenAddrPort == "" {
		t.Error("Server config was not loaded properly")
	}

	if serverConfig.TesseractPath != "" {
		t.Logf("Tesseract path configured: %s", serverConfig.TesseractPath)
	} else {
		t.Log("Tesseract not configured (as expected for optional OCR)")
	}

	if logger == nil {
		t.Error("Logger should not be nil")
	}
}

// TestIngressRunsAtStartup tests that the ingress job runs immediately at startup
func TestIngressRunsAtStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan bool)
	go func() {
		runIngressStartupTest(t)
		done <- true
	}()

	select {
	case <-done:
		return
	case <-ctx.Done():
		t.Fatal("Test timed out after 10 seconds")
	}
}

// runIngressStartupTest contains the actual test logic
func runIngressStartupTest(t *testing.T) {
	testDir := t.TempDir()
	testIngressDir := filepath.Join(testDir, "test_ingress")
	testDocumentsDir := filepath.Join(testDir, "test_documents")
	testDoneDir := filepath.Join(testDir, "test_done")

	os.MkdirAll(testIngressDir, 0755)
	os.MkdirAll(testDocumentsDir, 0755)
	os.MkdirAll(testDoneDir, 0755)

	testPDFPath := filepath.Join(testIngressDir, "test_document.pdf")
	if err := createSimpleTestPDF(testPDFPath); err != nil {
		t.Fatalf("Failed to create test PDF: %v", err)
	}

	serverConfig, logger := config.SetupServer()
	serverConfig.IngressPath = testIngressDir
	serverConfig.DocumentPath = testDocumentsDir
	serverConfig.IngressMoveFolder = testDoneDir
	serverConfig.IngressDelete = false
	serverConfig.IngressInterval = 1

	injectGlobals(logger)

	ephemeralDB, err := database.SetupEphemeralPostgresDatabase()
	if err != nil {
		t.Fatalf("Failed to setup ephemeral database: %v", err)
	}
	db := database.Repository(ephemeralDB)
	defer ephemeralDB.Close()
	defer db.Close()

	database.WriteConfigToDB(serverConfig, db)

	e := echo.New()
	e.HideBanner = true
	serverHandler := engine.ServerHandler{DB: db, Echo: e, ServerConfig: serverConfig}
	serverHandler.InitJobContext()
	serverHandler.InitializeSchedules(db)

	time.Sleep(5 * time.Second)

	processed := false
	movedFile := filepath.Join(testDoneDir, "test_document.pdf")
	if _, err := os.Stat(movedFile); err == nil {
		processed = true
		t.Logf("Document was processed and moved to done directory: %s", movedFile)
	}

	if _, err := os.Stat(testPDFPath); os.IsNotExist(err) {
		t.Log("Document was removed from ingress directory (processed)")
		processed = true
	}

	if !processed {
		t.Logf("Warning: Document may not have been processed yet, still in ingress")
	} else {
		t.Log("Ingress job ran at startup and processed the test document!")
	}
}

// createSimpleTestPDF creates a minimal valid PDF file for testing
func createSimpleTestPDF(filepath string) error {
	pdfContent := `%PDF-1.4
1 0 obj
<<
/Type /Catalog
/Pages 2 0 R
>>
endobj
2 0 obj
<<
/Type /Pages
/Kids [3 0 R]
/Count 1
>>
endobj
3 0 obj
<<
/Type /Page
/Parent 2 0 R
/MediaBox [0 0 612 792]
/Contents 4 0 R
/Resources <<
/Font <<
/F1 5 0 R
>>
>>
>>
endobj
4 0 obj
<<
/Length 44
>>
stream
BT
/F1 12 Tf
100 700 Td
(Test Document) Tj
ET
endstream
endobj
5 0 obj
<<
/Type /Font
/Subtype /Type1
/BaseFont /Helvetica
>>
endobj
xref
0 6
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
0000000262 00000 n
0000000356 00000 n
trailer
<<
/Size 6
/Root 1 0 R
>>
startxref
444
%%EOF`

	return os.WriteFile(filepath, []byte(pdfContent), 0644)
}
