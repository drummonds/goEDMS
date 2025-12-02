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
database/          # PostgreSQL migrations
internal/          # Shared internal packages
engine/            # Core document processing
static/            # Served static files (Swagger UI)
web/               # Built WASM output (app.wasm)
```

## Common Tasks

**Frontend changes**: Edit `webapp/*.go`, then `task build:wasm`

**CSS changes**: Edit `webapp/webapp.css` (single CSS file for all styles)

**Database changes**: Add migration in `database/migrations/`, follow existing naming

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
