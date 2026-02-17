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

Each document in the system consists of a root file and optional companion files:

```
document.pdf           # Root document (primary file)
document.txt           # Sidecar: extracted text content (optional)
document.tn_256.png    # Sidecar: thumbnail image (optional, for PDFs)
document.tags.json     # Sidecar: metadata including tags (optional)
```

**Rules:**

- Root documents: `.pdf`, `.jpg`, `.jpeg`, `.png`, `.tiff`, `.doc`, `.docx`, `.odf`, `.rtf`, `.text`
- `.txt` is ONLY a root document if no other source file exists with the same base name
- Use `.text` extension for primary text files to avoid ambiguity with sidecar `.txt` files
- `.txt` as sidecar: contains extracted/OCR'd text from the root document
- `.tn_256.png`: uniform thumbnail generated from first page with page-count watermark
- `.tags.json`: JSON file with tags and metadata

**Clean Database behavior:**

- Removes DB entries for files that no longer exist on disk
- Rescans orphaned files (on disk but not in DB) in-place
- Skips duplicate files by hash (first occurrence wins)
- Regenerates missing sidecar `.txt` files from DB
- Generates missing thumbnails for PDFs
- Skips `.txt` files that are sidecars (have a corresponding root document)
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

External tools (e.g. Evernote importer) can upload documents and enrich them with metadata:

```bash
# 1. Upload the document file
curl -X POST http://localhost:8000/api/document/upload \
  -F "file=@document.pdf"

# 2. Look up the document by its MD5 hash to get the ULID
HASH=$(md5sum document.pdf | cut -d' ' -f1)
curl http://localhost:8000/api/document/lookup?hash=$HASH
# Returns: {"ulid":"01J...", "name":"document.pdf", ...}

# 3. Set import metadata (all fields optional)
curl -X PUT http://localhost:8000/api/document/$ULID/metadata \
  -H 'Content-Type: application/json' \
  -d '{"author":"John","source":"evernote","source_url":"https://...","created_date":"2024-03-15T10:30:00Z"}'
# Also generates a thumbnail if missing

# 4. Add tags
curl -X POST http://localhost:8000/api/documents/$ULID/tags \
  -H 'Content-Type: application/json' \
  -d '{"tag_id": 5}'
```

Key endpoints:
- `GET /api/document/lookup?hash=<md5>` — find document by file hash (returns full document JSON)
- `PUT /api/document/:id/metadata` — set import metadata + auto-generate thumbnail
- `POST /api/documents/:ulid/tags` — add a tag to a document
- `GET /api/tags` — list all tags (to find tag IDs)

## See Also
- `.claude/rules.md` - Detailed coding guidelines and review modes
- `ARCHITECTURE.md` - System design
- `DATABASE_README.md` - Schema details
