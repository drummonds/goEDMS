# Testing Documentation

This document describes the test suite for goEDMS.

## Test Structure

The project includes both unit tests and integration tests:

### Unit Tests

**Located in: `config/config_test.go`**

Tests for configuration handling:
- `TestCheckExecutables_ValidPath` - Verifies that valid executable paths are accepted
- `TestCheckExecutables_InvalidPath` - Verifies that invalid paths return errors gracefully

**Located in: `engine/engine_test.go`**

Tests for engine resilience:
- `TestIngressDocumentNilPointerResilience` - Documents panic recovery and nil pointer protection

### Integration Tests

Located in: `main_test.go`

- `TestTesseractOptional` - Verifies that the application runs without Tesseract configured
- `TestFrontendRendering` - Tests that the frontend loads correctly using a headless browser
- `TestIngressRunsAtStartup` - Verifies that the ingress job runs immediately at startup and processes documents

## Running Tests

### Run All Tests

```bash
go test ./... -v
```

### Run Specific Tests

```bash
# Run only unit tests
go test ./config -v

# Run only integration tests
go test -v -run TestTesseractOptional
go test -v -run TestFrontendRendering
```

### Run Tests with Short Mode

Skip integration tests:

```bash
go test ./... -short
```

## Frontend Browser Tests

The `TestFrontendRendering` test supports multiple browsers and fallback options:

### Supported Test Methods (in order of preference):

1. **Chrome/Chromium** - Full headless browser testing with JavaScript support
2. **Firefox** - Falls back to curl/lynx (Firefox headless with chromedp is unreliable)
3. **Lynx** - Text-based browser for basic connectivity testing
4. **curl** - Simple HTTP client for basic connectivity testing

The test will automatically detect which tools are available and use the most appropriate one.

### Installing Test Tools:

```bash
# Ubuntu/Debian - Install Chrome/Chromium
sudo apt-get install chromium-browser

# Or use Google Chrome
sudo apt-get install google-chrome-stable

# Install curl (usually pre-installed)
sudo apt-get install curl

# Install lynx (optional)
sudo apt-get install lynx
```

**Note:** If only curl or lynx are available, the test will verify basic connectivity but won't test JavaScript functionality.

## Test Coverage

To generate test coverage:

```bash
go test ./... -cover
```

For detailed coverage report:

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Continuous Integration

The tests are designed to work in CI environments where Chrome may not be available - they will skip gracefully.

## Key Features Tested

1. **Tesseract Optional Functionality**
   - Application starts without valid Tesseract path
   - OCR functionality is disabled gracefully
   - No crashes or fatal errors

2. **Frontend Rendering** (when Chrome is available)
   - Server starts and serves static files
   - Frontend page loads
   - React application initializes

3. **Configuration Validation**
   - Executable path checking
   - Error handling for missing executables
   - Graceful degradation

4. **Ingress Job at Startup**
   - Ingress job runs immediately when server starts
   - Documents are processed without waiting for interval
   - Uses isolated test directories to avoid conflicts with dev documents
   - Verifies document processing (PDF → documents directory → done directory)
