# Changelog

All notable changes to goEDMS will be documented in this file.

## 0.7.0 2025-10-25

- Converting to Postgres full text search

## 0.6.0 2025-10-24

### Changed
- **BREAKING**: Replaced Bleve full-text search with PostgreSQL native full-text search
  - Simpler architecture with fewer external dependencies
  - Automatic index updates via PostgreSQL triggers
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
  - Simplified environment variable names (no more `GOEDMS_` prefix needed)
  - Old: `GOEDMS_DATABASE_HOST` → New: `DATABASE_HOST`
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
   DATABASE_NAME=goedms
   DATABASE_USER=your_user
   DATABASE_PASSWORD=your_password
   DATABASE_SSLMODE=disable

   # OCR (required)
   TESSERACT_PATH=/usr/bin/tesseract

   # Other settings (optional, have defaults)
   # See config.env for all available options
   ```

3. **Remove old TOML prefixes:**
   - Old `.env` had `GOEDMS_DATABASE_HOST`
   - New `.env` uses `DATABASE_HOST`
   - No prefix needed anymore!

4. **The old `serverConfig.toml` is no longer used**
   - You can delete it or keep as reference
   - All config is now in `.env` files

## [Previous Versions]

See git history for older changes.
