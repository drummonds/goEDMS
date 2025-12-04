# CLAUDE.md - Project Quick Reference

## What is godocs?
Document management system with Go backend, WebAssembly frontend (go-app), and PostgreSQL.

## Key Commands
```bash
task build              # Build backend binary
task build:wasm         # Build WASM frontend
task test               # Run all tests
task swagger:download   # Download Swagger UI assets
./godocs                # Run the server (reads .env)
```

## Project Structure
```
cmd/webapp/        # WASM frontend entry point
webapp/            # Frontend components (go-app)
webapp/webapp.css  # All frontend styles
database/          # Bun ORM migrations (in bun_migrations.go)
internal/          # Shared internal packages
engine/            # Core document processing
static/            # Served static files (Swagger UI)
web/               # Built WASM output (app.wasm)
```

## Document File Structure
Each document in the system consists of a root file and optional companion files:

```
document.pdf           # Root document (primary file)
document.txt           # Sidecar: extracted text content (optional)
document.tn_64.png     # Sidecar: thumbnail image (optional, for PDFs)
document.tags.json     # Sidecar: metadata including tags (optional)
```

**Rules:**
- Root documents: `.pdf`, `.jpg`, `.jpeg`, `.png`, `.tiff`, `.doc`, `.docx`, `.odf`, `.rtf`, `.text`
- `.txt` is ONLY a root document if no other source file exists with the same base name
- Use `.text` extension for primary text files to avoid ambiguity with sidecar `.txt` files
- `.txt` as sidecar: contains extracted/OCR'd text from the root document
- `.tn_64.png`: thumbnail generated from PDF first page
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

**Database changes**: Add migration function in `database/bun_migrations.go`

**API changes**: Update handlers in `main.go`, regenerate with `task openapi`

## Testing
- Tests use ephemeral PostgreSQL instances (auto-created/destroyed)
- Run specific test: `go test -run TestName -v`
- Integration tests: `go test -tags=integration`

## Environment
- Config via `.env` file (see `.env.example`)
- Frontend API URL configured via `BuildAPIURL()` in webapp

## See Also
- `.claude/rules.md` - Detailed coding guidelines and review modes
- `ARCHITECTURE.md` - System design
- `DATABASE_README.md` - Schema details
