# CLAUDE.md - Project Quick Reference

## What is godocs?
Document management system with Go backend, WebAssembly frontend (go-app), and PostgreSQL.

## Key Commands

```bash
task build              # Build backend binary
task test               # Run all tests
task swagger:download   # Download Swagger UI assets
./godocs                # Run the server (reads .env)
```

## Project Structure

```
internal/          # Shared internal packages
engine/            # Core document processing
static/            # Served static files (Swagger UI)
```

## Document File Structure

Each document uses canonical naming based on the DB auto-increment ID:

```
L/00/12/34/001234.orig.pdf     # Original document
L/00/12/34/001234.ocr.txt      # OCR/extracted text (optional)
L/00/12/34/001234.thumb.png    # Thumbnail image (optional)
L/00/12/34/001234.tags.json    # Tags metadata (optional)
```

**Rules:**

- Root documents: `.pdf`, `.jpg`, `.jpeg`, `.png`, `.tiff`, `.doc`, `.docx`, `.odf`, `.rtf`, `.text`
- ID padding matches tier: 6-digit (L), 8-digit (K), 10-digit (J)
- DB `Name` field preserves the original filename for display
- `.orig.` unambiguously marks the primary document
- `.ocr.txt`: extracted/OCR text sidecar
- `.thumb.png`: uniform thumbnail generated from first page
- `.tags.json`: JSON file with tags and metadata
- See `docs/internal/file-naming.md` for full details

**Clean Database behavior:**

- Removes DB entries for files that no longer exist on disk
- Rescans orphaned files (on disk but not in DB) in-place
- Skips duplicate files by hash (first occurrence wins)
- Regenerates missing `.ocr.txt` sidecar files from DB
- Generates missing thumbnails for supported types
- Migrates legacy file naming to canonical naming
- Deletes orphaned sidecar files (sidecars without a root document)

## Common Tasks

**Frontend changes**: Edit `webapp/*.go`, then `task build:wasm`

**CSS changes**: Edit `webapp/webapp.css` (single CSS file for all styles)


**API changes**: Update handlers in `main.go`, regenerate with `task openapi`

## Testing
- Tests use ephemeral PostgreSQL instances (auto-created/destroyed)
- Run specific test: `go test -run TestName -v`
- Integration tests: `go test -tags=integration`

## Environment
- Config via `.env` file (see `.env.example`)
- Frontend API URL configured via `BuildAPIURL()` in webapp

## Import API Workflow

See `docs/internal/agents.md` for full external agent/uploader instructions.

Key flow: upload → lookup by hash → set OCR text → set metadata → add tags.

Key endpoints:
- `POST /api/document/upload` — upload a document (rejects sidecar files)
- `GET /api/document/lookup?hash=<md5>` — find document by file hash
- `PUT /api/document/:id/ocr` — set OCR text (writes sidecar + updates DB + search index)
- `PUT /api/document/:id/metadata` — set import metadata + auto-generate thumbnail
- `POST /api/documents/:ulid/tags` — add a tag to a document
- `GET /api/tags` — list all tags (to find tag IDs)

## See Also
- `.claude/rules.md` - Detailed coding guidelines and review modes
- `ARCHITECTURE.md` - System design
- `DATABASE_README.md` - Schema details
- `docs/internal/agents.md` - External uploader agent instructions
- `docs/internal/file-naming.md` - Canonical file naming convention
- `docs/internal/tagging.md` - Tagging system, groups, stories, dimensions, external API
- `docs/internal/lifecycle.md` - Document lifecycle: ingestion, editing, archival
