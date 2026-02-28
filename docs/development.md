# Development

## Prerequisites

- Go 1.22 or later
- [Task](https://taskfile.dev/installation/) (build automation)
- PostgreSQL (for production mode, optional for development)
- Tesseract OCR (optional, for document OCR)

## Building from Source

```bash
# Build everything (backend + swagger assets + openapi spec)
task build

# Build backend only
task build:backend

# Build WASM demo
task build:wasm
```

## Task Commands Reference

**Development:**
- `task dev` — run the application locally
- `task demo` — run with pglike database and generated test documents

**Testing:**
- `task test` — run all Go tests
- `task test:coverage` — run tests with coverage report (generates HTML)
- `task test:race` — run tests with race detector
- `task test:api` — run API endpoint tests

**Building:**
- `task build` — build the full application
- `task build:backend` — build backend only
- `task build:wasm` — build WASM demo binary

**Code Quality:**
- `task fmt` — format Go code
- `task vet` — run go vet
- `task check` — run fmt, vet, and tests

**Documentation:**
- `task openapi` — generate OpenAPI specification
- `task docs:build` — rebuild all generated documentation
- `task docs:md2html` — convert internal markdown docs to HTML
- `task pages` — serve docs locally for GitHub Pages preview

**Cleanup:**
- `task clean` — remove build artifacts
- `task clean:all` — remove all generated files including Swagger UI

**Release:**
- `task release VERSION=v0.x.x COMMENT='description'` — full release process

## Running Tests

```bash
# All tests
task test

# Specific test suites
go test -v -run TestSearch              # Search functionality
go test -v -run TestSearchEndpoint      # API endpoints
go test -v -run TestSearchPerformance   # Performance tests
```

Tests use ephemeral PostgreSQL instances (auto-created/destroyed). No database setup needed.

## Docker

```bash
task docker:build    # Build Docker image
task docker:run      # Run container on port 8000
```

## gokrazy Deployment

godocs can be deployed to [gokrazy](https://gokrazy.org/), a pure Go appliance platform for Raspberry Pi and other devices.

The binary is self-contained with all static assets embedded. You only need to:

1. Add the godocs binary to your gokrazy instance
2. Provide a configuration file at `/etc/godocs.env` or use environment variables

```bash
# Add godocs to your gokrazy instance
gok add github.com/drummonds/godocs@latest

# Update and deploy
gok update
```

For a complete gokrazy setup example, see: [github.com/drummonds/gokrazy-godocs](https://github.com/drummonds/gokrazy-godocs)

## Performance

Ingestion speed is approximately 4 seconds per document on a typical laptop.
