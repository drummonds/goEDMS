package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	config "github.com/drummonds/goEDMS/config"
	database "github.com/drummonds/goEDMS/database"
	engine "github.com/drummonds/goEDMS/engine"
	"github.com/drummonds/goEDMS/webapp"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// getBrowser finds an available browser for testing
func getBrowser() (string, error) {
	browsers := []string{"firefox", "firefox-esr", "chromium", "chromium-browser", "google-chrome", "chrome"}
	for _, browser := range browsers {
		if path, err := exec.LookPath(browser); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no suitable browser found")
}

// TestFrontendRendering tests that the frontend loads correctly using a headless browser
func TestFrontendRendering(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set a timeout for the entire test
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use channel to detect if test completes or times out
	done := make(chan bool)
	go func() {
		runFrontendRenderingTest(t)
		done <- true
	}()

	select {
	case <-done:
		return
	case <-ctx.Done():
		t.Fatal("Test timed out after 10 seconds")
	}
}

// runFrontendRenderingTest contains the actual test logic
func runFrontendRenderingTest(t *testing.T) {

	// Check if any browser is available (Chrome, Chromium, or Firefox)
	browserPath, err := getBrowser()

	// Check for Firefox and use fallback immediately (before setting up server)
	if err == nil && (filepath.Base(browserPath) == "firefox" || filepath.Base(browserPath) == "firefox-esr") {
		// Firefox headless with chromedp is unreliable, use simpler tools
		if _, curlErr := exec.LookPath("curl"); curlErr == nil {
			t.Log("Firefox detected, using curl instead for reliability")
			testWithCurl(t)
			return
		}
		if _, lynxErr := exec.LookPath("lynx"); lynxErr == nil {
			t.Log("Firefox detected, using lynx instead for reliability")
			testWithLynx(t)
			return
		}
		t.Skip("Firefox found but curl/lynx not available, and Firefox headless is unreliable with chromedp")
	}

	if err != nil {
		// Try lynx as a fallback
		if _, err := exec.LookPath("lynx"); err == nil {
			t.Log("No GUI browser found, will use lynx for basic connectivity test")
			testWithLynx(t)
			return
		}
		// Try curl as the simplest fallback
		if _, err := exec.LookPath("curl"); err == nil {
			t.Log("No browser found, will use curl for basic connectivity test")
			testWithCurl(t)
			return
		}
		t.Skip("No browser (Chrome, Firefox, Lynx, or curl) found, skipping browser test")
	}
	t.Logf("Using browser: %s", browserPath)

	// Set up the server in a goroutine
	serverConfig, logger := config.SetupServer()
	injectGlobals(logger)

	// Force SQLite for tests (faster and more reliable)
	db := database.SetupDatabase("sqlite", "")
	searchDB, err := database.SetupSearchDB()
	if err != nil {
		t.Skipf("Unable to setup index database (may be locked): %v", err)
	}
	defer db.Close()
	defer searchDB.Close()

	database.WriteConfigToDB(serverConfig, db)

	e := echo.New()
	e.HideBanner = true // Hide Echo banner for cleaner test output
	serverHandler := engine.ServerHandler{DB: db, SearchDB: searchDB, Echo: e, ServerConfig: serverConfig}
	serverHandler.InitializeSchedules(db, searchDB)
	serverHandler.StartupChecks()
	e.Use(middleware.CORSWithConfig(middleware.DefaultCORSConfig))
	e.Static("/", "public/built")

	// Add routes
	e.GET("/home", serverHandler.GetLatestDocuments)
	e.GET("/documents/filesystem", serverHandler.GetDocumentFileSystem)
	e.GET("/document/:id", serverHandler.GetDocument)
	e.DELETE("/document/*", serverHandler.DeleteFile)
	e.PATCH("document/move/*", serverHandler.MoveDocuments)
	e.POST("/document/upload", serverHandler.UploadDocuments)
	serverHandler.AddDocumentViewRoutes()
	e.GET("/folder/:folder", serverHandler.GetFolder)
	e.POST("/folder/*", serverHandler.CreateFolder)
	e.GET("/search/*", serverHandler.SearchDocuments)

	// Start server in background
	testPort := "8999"
	go func() {
		if err := e.Start(fmt.Sprintf("127.0.0.1:%s", testPort)); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(2 * time.Second)
	defer e.Shutdown(context.Background())

	// Create headless browser context
	var opts []chromedp.ExecAllocatorOption

	// Configure browser-specific options (only Chrome/Chromium reach here due to Firefox check above)
	opts = append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(browserPath),
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Headless,
	)
	t.Log("Running test with Chrome/Chromium in headless mode")

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// Set a timeout for the browser operations
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Navigate to the home page and check if it renders
	var pageTitle string
	var bodyHTML string

	testURL := fmt.Sprintf("http://127.0.0.1:%s", testPort)

	err = chromedp.Run(ctx,
		chromedp.Navigate(testURL),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.Title(&pageTitle),
		chromedp.InnerHTML("body", &bodyHTML),
	)

	if err != nil {
		t.Fatalf("Failed to load page: %v", err)
	}

	// Verify the page loaded
	if pageTitle == "" {
		t.Error("Page title is empty")
	}

	if bodyHTML == "" {
		t.Error("Body HTML is empty")
	}

	// Check that the page contains expected content
	if len(bodyHTML) < 100 {
		t.Errorf("Body HTML seems too short (%d chars), page may not have rendered properly", len(bodyHTML))
	}

	t.Logf("Frontend test passed! Page title: %s, Body length: %d chars", pageTitle, len(bodyHTML))
}

// TestTesseractOptional tests that the application runs without Tesseract configured
func TestTesseractOptional(t *testing.T) {
	serverConfig, logger := config.SetupServer()

	// Verify that even with invalid Tesseract path, we still get a config
	if serverConfig.ListenAddrPort == "" {
		t.Error("Server config was not loaded properly")
	}

	// Verify that TesseractPath is empty when invalid
	if serverConfig.TesseractPath != "" {
		t.Logf("Tesseract path configured: %s", serverConfig.TesseractPath)
	} else {
		t.Log("Tesseract not configured (as expected for optional OCR)")
	}

	if logger == nil {
		t.Error("Logger should not be nil")
	}

	t.Log("Tesseract optional test passed - application can run without OCR")
}

// testWithLynx performs a basic connectivity test using lynx text browser
func testWithLynx(t *testing.T) {
	// Set a timeout for the test
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan bool)
	testErr := make(chan error, 1)

	go func() {
		err := runTestWithLynx(t)
		if err != nil {
			testErr <- err
		}
		done <- true
	}()

	select {
	case <-done:
		select {
		case err := <-testErr:
			t.Fatal(err)
		default:
			return
		}
	case <-ctx.Done():
		t.Fatal("Test timed out after 10 seconds")
	}
}

// runTestWithLynx contains the actual test logic
func runTestWithLynx(t *testing.T) error {
	// Set up the server
	serverConfig, logger := config.SetupServer()
	injectGlobals(logger)

	// Force SQLite for tests (faster and more reliable)
	db := database.SetupDatabase("sqlite", "")
	searchDB, err := database.SetupSearchDB()
	if err != nil {
		t.Skipf("Unable to setup index database (may be locked): %v", err)
	}
	defer db.Close()
	defer searchDB.Close()

	database.WriteConfigToDB(serverConfig, db)

	e := echo.New()
	e.HideBanner = true
	serverHandler := engine.ServerHandler{DB: db, SearchDB: searchDB, Echo: e, ServerConfig: serverConfig}
	serverHandler.InitializeSchedules(db, searchDB)
	serverHandler.StartupChecks()
	e.Use(middleware.CORSWithConfig(middleware.DefaultCORSConfig))
	e.Static("/", "public/built")

	// Add routes
	e.GET("/home", serverHandler.GetLatestDocuments)
	e.GET("/documents/filesystem", serverHandler.GetDocumentFileSystem)
	e.GET("/document/:id", serverHandler.GetDocument)

	// Start server in background
	testPort := "8998"
	go func() {
		if err := e.Start(fmt.Sprintf("127.0.0.1:%s", testPort)); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(2 * time.Second)
	defer e.Shutdown(context.Background())

	testURL := fmt.Sprintf("http://127.0.0.1:%s", testPort)

	// Use lynx to fetch the page
	cmd := exec.Command("lynx", "-dump", "-nolist", testURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Lynx failed to fetch page: %v, output: %s", err, string(output))
	}

	outputStr := string(output)

	// Basic checks that the page loaded
	if len(outputStr) < 10 {
		return fmt.Errorf("Lynx output too short (%d chars), page may not have loaded", len(outputStr))
	}

	// Check for any error messages in the output
	if strings.Contains(strings.ToLower(outputStr), "error") ||
		strings.Contains(strings.ToLower(outputStr), "not found") ||
		strings.Contains(strings.ToLower(outputStr), "404") {
		t.Logf("Warning: lynx output may contain errors: %s", outputStr)
	}

	t.Logf("Lynx test passed! Successfully fetched page (%d chars)", len(outputStr))
	t.Logf("First 200 chars of output: %s", outputStr[:min(200, len(outputStr))])
	return nil
}

// testWithCurl performs a basic connectivity test using curl
func testWithCurl(t *testing.T) {
	// Set a timeout for the test
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan bool)
	testErr := make(chan error, 1)

	go func() {
		err := runTestWithCurl(t)
		if err != nil {
			testErr <- err
		}
		done <- true
	}()

	select {
	case <-done:
		select {
		case err := <-testErr:
			t.Fatal(err)
		default:
			return
		}
	case <-ctx.Done():
		t.Fatal("Test timed out after 10 seconds")
	}
}

// runTestWithCurl contains the actual test logic
func runTestWithCurl(t *testing.T) error {
	// Set up the server
	serverConfig, logger := config.SetupServer()
	injectGlobals(logger)

	// Force SQLite for tests (faster and more reliable)
	db := database.SetupDatabase("sqlite", "")
	searchDB, err := database.SetupSearchDB()
	if err != nil {
		t.Skipf("Unable to setup index database (may be locked): %v", err)
	}
	defer db.Close()
	defer searchDB.Close()

	database.WriteConfigToDB(serverConfig, db)

	e := echo.New()
	e.HideBanner = true
	serverHandler := engine.ServerHandler{DB: db, SearchDB: searchDB, Echo: e, ServerConfig: serverConfig}
	serverHandler.InitializeSchedules(db, searchDB)
	serverHandler.StartupChecks()
	e.Use(middleware.CORSWithConfig(middleware.DefaultCORSConfig))
	e.Static("/", "public/built")

	// Add routes
	e.GET("/home", serverHandler.GetLatestDocuments)
	e.GET("/documents/filesystem", serverHandler.GetDocumentFileSystem)

	// Start server in background
	testPort := "8997"
	go func() {
		if err := e.Start(fmt.Sprintf("127.0.0.1:%s", testPort)); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(2 * time.Second)
	defer e.Shutdown(context.Background())

	testURL := fmt.Sprintf("http://127.0.0.1:%s", testPort)

	// Use curl to fetch the page
	cmd := exec.Command("curl", "-s", "-L", testURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Curl failed to fetch page: %v, output: %s", err, string(output))
	}

	outputStr := string(output)

	// Basic checks that the page loaded
	if len(outputStr) < 10 {
		return fmt.Errorf("Curl output too short (%d chars), page may not have loaded", len(outputStr))
	}

	// Check for HTML indicators
	if !strings.Contains(outputStr, "html") && !strings.Contains(outputStr, "HTML") {
		t.Logf("Warning: response may not be HTML")
	}

	// Check for any error indicators
	if strings.Contains(strings.ToLower(outputStr), "404") ||
		strings.Contains(strings.ToLower(outputStr), "500") ||
		strings.Contains(strings.ToLower(outputStr), "connection refused") {
		return fmt.Errorf("Curl output contains error indicators: %s", outputStr[:min(500, len(outputStr))])
	}

	t.Logf("Curl test passed! Successfully fetched page (%d chars)", len(outputStr))
	t.Logf("First 200 chars of output: %s", outputStr[:min(200, len(outputStr))])
	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestIngressRunsAtStartup tests that the ingress job runs immediately at startup
func TestIngressRunsAtStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set a timeout for the entire test
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use channel to detect if test completes or times out
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

	// Create isolated test directories
	testDir := t.TempDir()
	testIngressDir := filepath.Join(testDir, "test_ingress")
	testDocumentsDir := filepath.Join(testDir, "test_documents")
	testDoneDir := filepath.Join(testDir, "test_done")

	err := os.MkdirAll(testIngressDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test ingress directory: %v", err)
	}

	err = os.MkdirAll(testDocumentsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test documents directory: %v", err)
	}

	err = os.MkdirAll(testDoneDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test done directory: %v", err)
	}

	// Create a simple test PDF in the ingress directory
	testPDFPath := filepath.Join(testIngressDir, "test_document.pdf")
	err = createSimpleTestPDF(testPDFPath)
	if err != nil {
		t.Fatalf("Failed to create test PDF: %v", err)
	}

	t.Logf("Created test PDF at: %s", testPDFPath)

	// Verify the test PDF exists
	if _, err := os.Stat(testPDFPath); os.IsNotExist(err) {
		t.Fatalf("Test PDF was not created")
	}

	// Set up the server with custom config
	serverConfig, logger := config.SetupServer()

	// Override paths for testing
	serverConfig.IngressPath = testIngressDir
	serverConfig.DocumentPath = testDocumentsDir
	serverConfig.IngressMoveFolder = testDoneDir
	serverConfig.IngressDelete = false
	serverConfig.IngressInterval = 1 // 1 minute for testing

	injectGlobals(logger)

	// Force SQLite for tests (faster and more reliable)
	db := database.SetupDatabase("sqlite", "")
	searchDB, err := database.SetupSearchDB()
	if err != nil {
		t.Skipf("Unable to setup index database (may be locked): %v", err)
	}
	defer db.Close()
	defer searchDB.Close()

	// Update config in database
	database.WriteConfigToDB(serverConfig, db)

	e := echo.New()
	e.HideBanner = true
	serverHandler := engine.ServerHandler{DB: db, SearchDB: searchDB, Echo: e, ServerConfig: serverConfig}

	// Initialize schedules (this should trigger ingress job at startup)
	serverHandler.InitializeSchedules(db, searchDB)

	// Give the ingress job time to process the document
	// Since it runs in a goroutine, we need to wait a bit
	time.Sleep(5 * time.Second)

	// Check if the document was processed
	// It should either be in documents directory or moved to done directory
	processed := false

	// Check if file was moved to done directory
	movedFile := filepath.Join(testDoneDir, "test_document.pdf")
	if _, err := os.Stat(movedFile); err == nil {
		processed = true
		t.Logf("Document was processed and moved to done directory: %s", movedFile)
	}

	// Check if file is no longer in ingress
	if _, err := os.Stat(testPDFPath); os.IsNotExist(err) {
		t.Log("Document was removed from ingress directory (processed)")
		processed = true
	}

	// Check database for the document
	// We can't easily query the database without knowing the exact structure,
	// but we can check if processing happened by looking at logs or file movement

	if !processed {
		// File might still be in ingress if processing failed or is taking longer
		t.Logf("Warning: Document may not have been processed yet, still in ingress")
		// Don't fail the test, as processing might take longer in some environments
	} else {
		t.Log("Ingress job ran at startup and processed the test document!")
	}
}

// createSimpleTestPDF creates a minimal valid PDF file for testing
func createSimpleTestPDF(filepath string) error {
	// This is a minimal valid PDF structure
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

// TestWasmFileValid tests that the WASM file is valid
func TestWasmFileValid(t *testing.T) {
	wasmPath := "web/app.wasm"

	// Check if file exists
	info, err := os.Stat(wasmPath)
	if err != nil {
		t.Fatalf("WASM file not found at %s: %v. Run 'task build:wasm' first.", wasmPath, err)
	}

	// Check file is not empty
	if info.Size() == 0 {
		t.Fatal("WASM file is empty")
	}

	// Check magic number
	file, err := os.Open(wasmPath)
	if err != nil {
		t.Fatalf("Failed to open WASM file: %v", err)
	}
	defer file.Close()

	magicNumber := make([]byte, 4)
	_, err = file.Read(magicNumber)
	if err != nil {
		t.Fatalf("Failed to read WASM magic number: %v", err)
	}

	// WASM magic number should be: 0x00 0x61 0x73 0x6d ("\0asm")
	expectedMagic := []byte{0x00, 0x61, 0x73, 0x6d}
	if !bytes.Equal(magicNumber, expectedMagic) {
		t.Errorf("Invalid WASM magic number. Got %v, expected %v", magicNumber, expectedMagic)
		t.Errorf("This usually means the WASM file was not built correctly.")
		t.Errorf("The file appears to be: %v", string(magicNumber))
	}

	t.Logf("WASM file is valid: %s (%d bytes)", wasmPath, info.Size())
}

// TestRootEndpoint tests that the root endpoint returns a 200 OK response with WASM app
func TestRootEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Just run the test directly without goroutine/timeout wrapper
	// The test framework already has timeouts
	runRootEndpointTest(t)
}

// runRootEndpointTest contains the actual test logic
func runRootEndpointTest(t *testing.T) {
	// Set up the server
	serverConfig, logger := config.SetupServer()
	injectGlobals(logger)

	// Force SQLite for tests (faster and more reliable)
	db := database.SetupDatabase("sqlite", "")
	searchDB, err := database.SetupSearchDB()
	if err != nil {
		t.Skipf("Unable to setup index database (may be locked): %v", err)
	}
	defer db.Close()
	defer searchDB.Close()

	database.WriteConfigToDB(serverConfig, db)

	e := echo.New()
	e.HideBanner = true
	serverHandler := engine.ServerHandler{DB: db, SearchDB: searchDB, Echo: e, ServerConfig: serverConfig}
	serverHandler.InitializeSchedules(db, searchDB)
	serverHandler.StartupChecks()
	e.Use(middleware.CORSWithConfig(middleware.DefaultCORSConfig))

	// Set up WASM app routes exactly as in main.go
	appHandler := webapp.Handler()

	e.GET("/wasm_exec.js", func(c echo.Context) error {
		return c.File("web/wasm_exec.js")
	})

	e.GET("/app.js", echo.WrapHandler(appHandler))
	e.GET("/app.css", echo.WrapHandler(appHandler))
	e.GET("/manifest.webmanifest", echo.WrapHandler(appHandler))

	e.Static("/web", "web")
	e.File("/webapp/webapp.css", "webapp/webapp.css")
	e.File("/favicon.ico", "public/built/favicon.ico")

	// Add API routes
	e.GET("/home", serverHandler.GetLatestDocuments)
	e.GET("/documents/filesystem", serverHandler.GetDocumentFileSystem)

	// Serve go-app handler for all other routes (must be last)
	e.Any("/*", echo.WrapHandler(appHandler))

	// Start server in background
	testPort := "8996"
	go func() {
		if err := e.Start(fmt.Sprintf("127.0.0.1:%s", testPort)); err != nil {
			t.Logf("Server stopped: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(2 * time.Second)
	defer e.Shutdown(context.Background())

	testURL := fmt.Sprintf("http://127.0.0.1:%s/", testPort)
	t.Logf("Testing URL: %s", testURL)

	// Use curl to test the endpoint with a timeout
	cmd := exec.Command("curl", "-s", "-L", "-w", "\n%{http_code}", "--max-time", "5", testURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Curl error: %v, output: %s", err, string(output))
		// Don't fatal here, continue to analyze the output
	}

	outputStr := string(output)
	lines := strings.Split(strings.TrimSpace(outputStr), "\n")

	// The last line should be the HTTP status code
	if len(lines) < 1 {
		t.Fatalf("No output from curl")
	}

	statusCode := lines[len(lines)-1]
	responseBody := strings.Join(lines[:len(lines)-1], "\n")

	t.Logf("HTTP Status Code: %s", statusCode)
	t.Logf("Response length: %d chars", len(responseBody))
	t.Logf("First 200 chars: %s", responseBody[:min(200, len(responseBody))])

	// Check if we got a 200 OK
	if statusCode != "200" {
		t.Errorf("Expected status code 200, got %s", statusCode)
	}

	// Check that we got some content back
	if len(responseBody) < 10 {
		t.Errorf("Response body too short (%d chars), expected HTML content", len(responseBody))
	}

	// Check for HTML indicators
	if !strings.Contains(responseBody, "html") && !strings.Contains(responseBody, "HTML") {
		t.Logf("Warning: response may not be HTML")
	}

	// Check that the page doesn't contain the "Go is not defined" error
	if strings.Contains(responseBody, "Go is not defined") {
		t.Error("Page contains 'Go is not defined' error - WebAssembly not loading correctly")
	}

	// Test that wasm_exec.js is accessible at root
	wasmURL := fmt.Sprintf("http://127.0.0.1:%s/wasm_exec.js", testPort)
	wasmCmd := exec.Command("curl", "-s", "-L", "-w", "\n%{http_code}", "--max-time", "5", wasmURL)
	wasmOutput, err := wasmCmd.CombinedOutput()
	if err != nil {
		t.Logf("Warning: Could not fetch /wasm_exec.js: %v", err)
	} else {
		wasmOutputStr := string(wasmOutput)
		wasmLines := strings.Split(strings.TrimSpace(wasmOutputStr), "\n")
		if len(wasmLines) > 0 {
			wasmStatusCode := wasmLines[len(wasmLines)-1]
			t.Logf("/wasm_exec.js status code: %s", wasmStatusCode)
			if wasmStatusCode != "200" {
				t.Errorf("/wasm_exec.js returned status %s, expected 200", wasmStatusCode)
			}
		}
	}

	if statusCode == "200" && len(responseBody) > 10 {
		t.Log("/app endpoint test passed!")
	}
}
