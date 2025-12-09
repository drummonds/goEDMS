# Changelog

All notable changes to godocs will be documented in this file.



## 0.35.0 2025-12-09

- Adding search manager page
- removing stormid

## 0.34.10 2025-12-07

- Debug tag errors

## 0.34.9 2025-12-06

- Fixing edit tags errors

## 0.34.3 2025-12-05

- Adding search manager
- and bug fixes

## 0.34.2 2025-12-05

- Adding search manager
- and bug fixes

## 0.33.7 2025-12-04

- Adding tag manager
- Converting dimensions to tag groups
- clean_database now rescans orphaned files on demand
- removal dual db migration

## 0.32.6 2025-12-04

- Extending edit to show more info
- improving edit to regen thumbnail with button
- update appearance of tags

## 0.31.8 2025-12-03

- fixing view
- fixing edit
- looking at DB migations
- fixing file browser

## 0.31.3 2025-12-02

- Fixing front end port
- fixing port again!
- making URL address dynamic
- tidying main.go by creating handlers.go

## 0.30.2 2025-12-02

- Creating CSS classes for about page

## 0.29.6 2025-11-25

- Making backend url dynamic

## 0.29.3 2025-11-24

- fixing view document routes

## 0.28.5 2025-11-23

- Fix bugs on routing to backend api
- Change routes where URLS are to the backend from the front end eg swagger docs

## 0.28.0 2025-11-22

- **Major architectural change**: Single godocs startup now serves both ports simultaneously
- Backend (port 8000): Serves API endpoints + static files (WASM, CSS) + direct access
- Frontend (port 8001): Serves minimal HTML shell that loads resources from backend
- **Recommended usage**: Access app via http://localhost:8001 for clear frontend/backend separation
- WASM app makes direct API calls to backend (no proxy)
- Added improved CORS configuration for cross-origin requests
- Created separate server binaries (cmd/backend, cmd/frontend) for advanced use cases
- Created start-separated.sh script for running servers separately if needed
- Clear delineation between frontend routes and backend API endpoints

## 0.27.12 2025-11-22

- Manual test logging from Front End
- correcting swagger ui tests

## 0.27.7 2025-11-22

- Extending test coverage
- fixing forntend 404 errors not being logged
- fixing thumbnails not being displayed
- fixing edit routes
- replacing echo with slog log

## 0.26.2 2025-11-22

- embedding swagger rendering of api swagger.json
- rebulding release so cleans, builds and tests first

## 0.25.2 2025-11-22

- Using rendered API docs locally rather then externally

## 0.24.0 2025-11-21

- Adding backend API docs to About page (to test logging)

## 0.23.5 2025-11-20

- Refactoring logs so frontend is logged on backend

## 0.22.13 2025-11-19

- Trying to fix persistent 404 errors

## 0.22.12 2025-11-18

- Adding Tag editing system

## 0.21.4 2025-11-18

- Adding Tagging system

## 0.20.7 2025-11-18

- Fixing display bugs for thumbnails

## 0.20.0 2025-11-16

- using goreleaser release

## 0.19.0 2025-11-16

- Adding creating thumbnails

## 0.18.0 2025-11-16

- Adding ingres .txt files and store on clean if don't exist

## 0.16.0 2025-11-11

- Adding make ingress directory if it doesn't exist

## 0.15.0 2025-11-10

- Embedding static files into binary for single-file deployment
- Log version on startup
- Fixed build scripts to properly set version using internal/build.Version 

## 0.14.0 2025-11-10

- Adding /etc/godocs.env as env 

## 0.12.0 2025-11-08

- Converting to url github.com/drummonds/godocs as cache was corrupted

## 0.11.0 2025-11-08

- Converting to url github.com/drummonds/goems

## 0.10.0 2025-11-08

- Pure go renderer

## 0.9.0 2025-11-08

### Changed
- **BREAKING**: Removed go-fitz (CGo) PDF renderer in favor of pure Go PDFium renderer
  - All PDF rendering now uses PDFium with WebAssembly (no CGo required)
  - Works with `CGO_ENABLED=0` for simpler deployment
  - Single binary with no external C dependencies
  - Removed `PDF_RENDERER` configuration option
- Converted to Bun ORM for better database abstraction
  - Support for both SQLite and PostgreSQL
  - Unified interface across database types

### Added
- Pure Go PDF rendering via PDFium WebAssembly
  - No MuPDF or other C library dependencies required
  - Simplified deployment for Docker, cloud, and embedded systems

## 0.8.0

### Added
- **Word Cloud Visualization**: Automatic word frequency analysis for all documents
  - Real-time word cloud generation using PostgreSQL queries
  - Excludes common stop words for better visualization
  - Integrated into frontend with responsive display
- **Step-Based Ingestion Pipeline**: Complete refactoring of document ingestion
  - Step 1: Hash calculation and duplicate detection
  - Step 2: File move with hash verification
  - Step 3: Text extraction and search indexing
  - Comprehensive job tracking with per-file progress reporting
  - Graceful failure handling - documents saved even if OCR fails
- **Build System Improvements**:
  - Added `build-wasm.sh` script for WASM builds with version embedding
  - Fixed Taskfile.yml YAML syntax issues
  - Version information now properly embedded in WASM binary
- **Improved OCR Handling**:
  - Documents with no extractable text (handwritten, blank, etc.) now stored successfully
  - OCR failures return empty text instead of errors
  - Better logging for OCR issues

### Changed
- **BREAKING**: Configuration now stored in PostgreSQL database
  - Removed `config/serverConfig.toml` entirely
  - Only database connection uses `.env` file
  - All other settings (ingress paths, OCR, etc.) stored in database
  - Web interface for configuration management
- **BREAKING**: Default ingestion behavior changed
  - `INGRESS_DELETE=true` by default (deletes source files)
  - Removed "done" folder concept
  - Files are hashed, verified, then source deleted
- **Ingestion Storage**: Documents no longer duplicated in multiple folders
  - Single copy in documents folder with database tracking
  - MD5 hash verification before and after file copy

### Fixed
- Orphaned files when OCR fails now prevented
- Empty OCR results treated as valid (not errors)
- Test files no longer leak into production directories
- Taskfile.yml syntax errors causing build failures

### Documentation
- Updated README.md to remove outdated config.toml references
- Added comprehensive ingestion flow documentation
- Created BUILD.md with detailed build instructions
- Added INGESTION_REFACTOR.md documenting step-based approach

## 0.7.0 2025-10-25

### Added
- **WebAssembly Frontend**: Complete migration from React to go-app
  - Pure Go frontend compiled to WebAssembly
  - No npm dependencies needed
  - Embedded in backend binary
- **PostgreSQL Full-Text Search**: Replaced Bleve with native PostgreSQL
  - Automatic index updates via triggers
  - Better performance with GIN indexes
  - Single word, phrase, and prefix matching supported

## 0.6.0 2025-10-24

### Changed
- **BREAKING**: Replaced Bleve full-text search with PostgreSQL native full-text search
  - Simpler architecture with fewer external dependencies
  - Automatic index updates via PostgreSQL triggerConverting to Postgres full text searchs
  - Better performance with GIN indexes
  - No separate search index files needed
  - Search functionality preserved: single word, phrase, and prefix matching supported

### Removed
- Bleve search library and all related dependencies (~200KB+ removed)
- `database/searchDatabase.go` - no longer needed
  - `engine/search.go` - search logic moved to PostgreSQL
- `DeleteDocumentFromSearch` function - automatic via database triggers
- `SearchDB` field from ServerHandler struct

### Technical Details
- Added migration `000002_add_fulltext_search` for PostgreSQL tsvector support
- Search now uses `to_tsvector` and `to_tsquery` for English language text
- Implemented automatic trigger to update search index on document insert/update
- Added comprehensive test suite for search functionality (5 tests, all passing)
- Binary size: 28M (after removing Bleve dependencies)

## 0.5.0 2025-10-24

### Added
- Enhanced About page with detailed database connection information
  - Shows database host, port, database name
  - Displays connection type (ephemeral vs external)
  - Split configuration into separate Database and OCR sections
- Comprehensive test suite for About page
  - Backend API tests for `/api/about` endpoint
  - Client-side unit tests for AboutPage component
  - Integration tests with lynx (fast, route verification)
  - Integration tests with chromedp (full WASM rendering)
- Added `config.env` template file with all configuration options
- Added `CHANGELOG.md` for tracking project changes

### Changed
- **BREAKING**: Simplified configuration system
  - Removed Viper dependency (lighter, simpler)
  - Replaced `serverConfig.toml` with `.env` key=value format
  - Simplified environment variable names (no more `GODOCS_` prefix needed)
  - Old: `GODOCS_DATABASE_HOST` → New: `DATABASE_HOST`
  - Configuration now loads from: defaults → `config.env` → `.env` → environment variables
- Improved CSS spacing for h3 headings (more whitespace above)
- Updated `.env.example` to reflect new simplified variable names

### Fixed
- `.env` file support now actually works (was broken with Viper)
- About page route properly registered in WASM client
- Tests now properly detect 404 errors on About page

### Technical Details
- Configuration file reduced from 307 lines to 300 lines (same length, much simpler)
- Added `github.com/joho/godotenv` for .env file parsing (~11KB)
- Removed `github.com/spf13/viper` and dependencies (~200KB)
- Binary size: 28M
- All tests passing: config tests, webapp tests, integration tests

### Migration Guide
If upgrading from the old TOML configuration:

1. **Backup your old config:**
   ```bash
   cp config/serverConfig.toml config/serverConfig.toml.backup
   ```

2. **Create new `.env` file from your TOML settings:**
   ```env
   # Database (required)
   DATABASE_TYPE=postgres
   DATABASE_HOST=localhost
   DATABASE_PORT=5432
   DATABASE_NAME=godocs
   DATABASE_USER=your_user
   DATABASE_PASSWORD=your_password
   DATABASE_SSLMODE=disable

   # OCR (required)
   TESSERACT_PATH=/usr/bin/tesseract

   # Other settings (optional, have defaults)
   # See config.env for all available options
   ```

3. **Remove old TOML prefixes:**
   - Old `.env` had `GODOCS_DATABASE_HOST`
   - New `.env` uses `DATABASE_HOST`
   - No prefix needed anymore!

4. **The old `serverConfig.toml` is no longer used**
   - You can delete it or keep as reference
   - All config is now in `.env` files

## [Previous Versions]

See git history for older changes.
