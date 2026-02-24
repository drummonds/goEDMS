# godocs Research Report

## 1. Project Overview

godocs is a self-hosted Electronic Document Management System (EDMS) built in Go. It manages document scanning, OCR, organization, tagging, searching, and storage. Hard fork of deranjer/godocs with major modernization.

**Stack:** Go 1.25, Echo web framework, SSR via Pongo2 templates + Bulma CSS, PostgreSQL (production) / pglike/SQLite (dev/testing), optional Tesseract OCR, PDFium WASM for PDF rendering.

### Architecture

Single binary, single port (8000). SSR replaced an earlier WASM (go-app) frontend. All `/api/*` JSON routes preserved alongside HTML page routes.

```
main.go                  # Server entry, route setup, middleware
page_handlers.go         # SSR page handlers (home, search, document, tags, stories, jobs)
story_handlers.go        # Story CRUD handlers
bulk_edit_handlers.go    # Bulk operations
templates.go             # Pongo2 template system with embed.FS loader
templates/               # Pongo2 HTML templates (base.html + pages + partials)
engine/
  engine.go              # Core ingestion pipeline, file processing, cleanup
  ingestion_steps.go     # 3-step ingestion with progress tracking
  scheduler.go           # Cron-based ingestion scheduling
  routes.go              # API route handlers, ServerHandler struct
  routes_search.go       # Search API
  routes_tags.go         # Tag/dimension API
  jobs_routes.go         # Job tracking API
  nested_path.go         # Canonical file naming (L/K/J tiers)
  thumbnails.go          # Thumbnail generation
  rotate.go              # Document rotation
  tags_sidecar.go        # .tags.json sidecar handling
  startupChecks.go       # Health checks
  config_dir.go          # Config directory management
  pdfrenderer/           # PDFium WASM renderer
database/
  database.go            # Repository interface, Document struct
  pg_database.go         # PGDB implementation (raw SQL)
  pg_repository.go       # DB initialization
  pg_migrations.go       # 13 schema migrations
  pg_scan.go             # Row scanning
  pg_search.go           # Search execution (tsvector + LIKE fallback)
  pg_tags.go             # Tag SQL operations
  pg_stories.go          # Story SQL operations
  job.go                 # Job types and structs
  search.go              # ParsedSearch, query parsing
  tags.go                # Tag data structures
  stories.go             # Story data structures
  mem_database.go        # In-memory test implementation
config/
  config.go              # ServerConfig from env vars
internal/
  build/                 # Version injection
  docs/                  # Documentation helpers
  testdocs/              # Test fixtures
```

### Key Features

- **Document Ingestion:** 3-step pipeline (hash/dedup, move/verify, extract/index)
- **Canonical File Naming:** `L/00/12/34/001234.orig.pdf` with sidecars `.ocr.txt`, `.thumb.png`, `.tags.json`
- **Search:** Unified query syntax with text, `#tag`, `~exclude`, `before:`, `after:`, `!untagged`
- **Tags:** Free tags, grouped tags (one per group per doc), tag aliases, colors
- **Stories:** Document groups with start/end dates, implemented via special tag group
- **Job Tracking:** ULID-based async job system with progress reporting
- **Database Cleanup:** Multi-step process (orphan detection, rescan, thumbnail generation, canonical migration)

### Database

Repository pattern with two backends sharing the same SQL:
- PostgreSQL via `lib/pq` (production)
- pglike/go-postgres v0.3.0 via SQLite (dev/testing)

13 migrations covering: documents, full-text search (tsvector), word cloud, jobs, tags, dimensions, saved searches, stories, document metadata, hide tag, tag aliases.

---

## 2. Notification System

### Finding: No Formal Notification System

godocs has no user-facing notification/toast/flash message system. It uses a stateless SSR approach.

### What Exists

**Empty State Messages (Bulma `.notification` class):**
- `templates/home.html:63-65` — "No documents found"
- `templates/search.html:112-114` — "No search results"
- `templates/tags.html:100-102` — "No tags"
- `templates/jobs.html:83-85` — "No jobs"
- `templates/stories.html:26-28` — "No stories"

**Error Display (template context variables):**
- `templates/tags.html:8-10` — `{% if error %}` notification block
- `templates/tag_edit.html:8-10` — same pattern
- `templates/jobs.html:12-14` — same pattern

These only work when the handler passes an `error` key in the template context.

**Backend Pattern — Redirect Without Feedback:**
```go
// All POST handlers follow this pattern:
func HandleCreateTag(tr *TemplateRenderer) echo.HandlerFunc {
    return func(c echo.Context) error {
        // ... process form ...
        if err := tr.db.CreateTag(tag); err != nil {
            Logger.Error("Failed to create tag", "error", err)
            // Error is ONLY logged, not shown to user
        }
        return c.Redirect(http.StatusSeeOther, "/tags")
    }
}
```

Errors are logged server-side but never communicated to the user. Success is inferred from the redirected page showing updated content.

**API Responses:**
JSON endpoints include simple `"message"` fields: `"Tag added successfully"`, `"Thumbnail regenerated successfully"`, etc.

**Job Status as "Notifications":**
The jobs page (`templates/jobs.html`) is the closest thing to a notification system:
- Shows job status, progress bar, error messages
- Auto-refreshes every 3 seconds when jobs are active: `<meta http-equiv="refresh" content="3">`
- Buttons disabled while jobs run

**PushBullet Placeholder:**
`config/config.go:211` reads `PUSHBULLET_TOKEN` env var. Stored in DB (`server_config.pushbullet_token`). **Never used** — no sending code exists.

### No Session State

- No cookies for flash messages
- No session middleware
- No query parameter passing of errors between redirects
- No AJAX/SSE/WebSocket for real-time updates

---

## 3. Task Scheduling System — Bug Report

### Architecture Overview

The task scheduling system has three layers:

1. **Cron Scheduler** (`engine/scheduler.go`) — Periodic ingress scanning
2. **Manual Triggers** — SSR page buttons + API endpoints launch goroutines
3. **Job Tracking** (`database/job.go`, `pg_database.go`) — DB-backed progress/status

**Flow:**
1. `InitializeSchedules()` called at startup → creates `cron.New()`, adds ingress job, starts cron
2. Cron fires `ingressJobFunc()` periodically (no job tracking)
3. Manual triggers (`HandleTriggerIngest`, `HandleTriggerClean`, `RunIngestNow`, `CleanDatabase`) create a Job record then launch goroutine
4. Goroutines call `ingressJobFuncWithTracking()` or `cleanupJobFuncWithTracking()` which update job progress
5. Jobs page polls via auto-refresh, disables buttons while `has_active` is true

---

### Bug #1: Cancelled Jobs Execute to Completion (CRITICAL)

**Location:** `engine/engine.go:106-195` and `engine/engine.go:197-488`

The `JobStatusCancelled` constant exists (`database/job.go:17`) but job execution functions **never check** if the job has been cancelled. Once a goroutine is launched, it runs the entire pipeline regardless.

```go
// engine/engine.go:157-176 — NO cancellation check in the loop
for i, filePath := range ingressFiles {
    // MISSING: check if job status == cancelled
    err := serverHandler.IngestDocumentWithSteps(filePath, db, jobID, i, totalFiles)
    // ... continues processing all files
}
```

The same applies to `cleanupJobFuncWithTracking()` — its multi-step cleanup (9 steps) never checks for cancellation between steps.

**Impact:** If a job could somehow be marked cancelled (e.g. direct DB update), the goroutine would ignore it and keep processing.

### Bug #2: No Cancellation Mechanism Exists (CRITICAL)

**Location:** `database/database.go:80-89` (Repository interface)

The `Repository` interface has no `CancelJob()` method. There is:
- No API endpoint to cancel a job
- No UI button to cancel a job
- No code path that ever sets `JobStatusCancelled`

`JobStatusCancelled` is defined but only referenced in `DeleteOldJobs()` cleanup query. It is **never written** by any code path.

Combined with Bug #1, there is zero cancellation capability — neither the mechanism to request cancellation nor the mechanism to honour it.

### Bug #3: Cron Scheduler Never Stopped — Goroutine Leak (CRITICAL)

**Location:** `engine/scheduler.go:14-33`

```go
func (serverHandler *ServerHandler) InitializeSchedules(db database.Repository) {
    // ...
    go serverHandler.ingressJobFunc(serverConfig, db)  // Fire-and-forget goroutine

    c := cron.New()           // Created as local variable
    // ... add jobs ...
    c.Start()                 // Started, but `c` is never stored
}   // `c` goes out of scope — no reference kept
```

**Problems:**
1. `ServerHandler` struct has no `scheduler` field — `c` is never stored
2. `main.go` has no graceful shutdown — no `c.Stop()` call anywhere
3. The cron goroutine and its internal goroutines run until process exit
4. Cannot modify schedule at runtime (e.g. change ingestion interval)

### Bug #4: Duplicate Jobs Can Run Simultaneously (HIGH)

**Location:** `engine/routes.go:975-996` (`RunIngestNow`), `engine/routes.go:1008-1029` (`CleanDatabase`), `page_handlers.go:760-780` (`HandleTriggerClean`, `HandleTriggerIngest`)

None of the job-launching code checks whether a job of the same type is already running before creating a new one.

**UI mitigation exists but is incomplete:**
- `templates/jobs.html:20,25` disables buttons when `has_active` is true
- But this is only a client-side guard — the API endpoints have no such check
- A fast double-click, concurrent API calls, or cron + manual trigger can launch duplicates

**Race condition scenario:**
1. Cron fires `ingressJobFunc()` (no tracking, no active job record)
2. User clicks "Run Ingestion" → creates tracked job, launches goroutine
3. Both ingestion processes run concurrently on the same files
4. File moves, hash checks, and DB writes race against each other

The cron-based `ingressJobFunc()` has `SkipIfStillRunning` protection, but this only prevents cron-vs-cron overlap. It does NOT prevent cron-vs-manual or manual-vs-manual overlap.

### Bug #5: No Context-Based Cancellation (HIGH)

**Location:** All job functions in `engine/engine.go`

Job functions don't accept `context.Context`:

```go
func (serverHandler *ServerHandler) ingressJobFuncWithTracking(
    serverConfig config.ServerConfig,
    db database.Repository,
    jobID ulid.ULID,   // No context.Context
)
```

Without context:
- Cannot propagate cancellation signals
- Cannot enforce timeouts
- Cannot cancel downstream operations (DB queries, file I/O)
- Cannot participate in server shutdown

### Bug #6: Startup Ingestion Job Not Tracked (MEDIUM)

**Location:** `engine/scheduler.go:21-23`

```go
Logger.Info("Running ingress job at startup")
go serverHandler.ingressJobFunc(serverConfig, db)  // Uses NON-tracking version
```

The startup job calls `ingressJobFunc()` instead of `ingressJobFuncWithTracking()`:
- No Job record created in DB
- No progress tracking
- Not visible on jobs page
- No error recording if it fails
- `has_active` check won't see it → user can trigger a manual ingestion that overlaps

### Bug #7: Cron References Stale Config (MEDIUM)

**Location:** `engine/scheduler.go:27`

```go
ingressJob = cron.FuncJob(func() {
    serverHandler.ingressJobFunc(serverConfig, db)  // Captures serverConfig by value
})
```

`serverConfig` is captured by value at initialization time. The function then re-fetches config from DB (`engine/engine.go:68`), which shadows the parameter. This is confusing but currently harmless because the re-fetch overrides. However:

- The cron *schedule* (`@every Xm`) is fixed at startup — changing `IngressInterval` in config requires a server restart
- The function signature accepts a `serverConfig` parameter that is immediately discarded

### Bug #8: Panic Recovery Leaves Jobs Stuck in "running" (MEDIUM)

**Location:** `engine/engine.go:108-113`, `engine/engine.go:199-204`

```go
defer func() {
    if r := recover(); r != nil {
        Logger.Error("Panic recovered in ingress job", "panic", r, "jobID", jobID)
        db.UpdateJobError(jobID, fmt.Sprintf("Panic: %v", r))
    }
}()
```

If `db.UpdateJobError()` itself fails (DB connection lost, for example), the job remains in `running` status forever. The `GetActiveJobs()` query will return it indefinitely, which:
- Keeps the jobs page auto-refreshing forever
- Keeps action buttons disabled forever
- Blocks the UI duplicate-prevention guard

### Bug #9: `has_active` Includes Stale Pending Jobs (LOW)

**Location:** `database/pg_database.go:576-585`

```go
func (p *PGDB) GetActiveJobs() ([]Job, error) {
    rows, err := p.db.QueryContext(context.Background(),
        `SELECT ... FROM jobs WHERE status IN ($1, $2) ...`,
        JobStatusPending, JobStatusRunning)
}
```

Jobs are created as `pending` then immediately moved to `running` when the goroutine starts. But if the goroutine panics before calling `UpdateJobStatus(jobID, JobStatusRunning, ...)`, the job stays `pending` forever. Combined with Bug #8, any orphaned pending/running job permanently disables the UI action buttons.

### Bug #10: Manual Ingestion Bypasses `SkipIfStillRunning` (LOW)

**Location:** `engine/scheduler.go:28` vs `engine/routes.go:987-990`

The cron job uses `cron.SkipIfStillRunning` to prevent overlapping cron executions. But manual triggers (`RunIngestNow`, `HandleTriggerIngest`) launch separate goroutines that bypass this protection entirely. The `SkipIfStillRunning` wrapper only guards the cron's internal goroutine pool.

---

### Summary Table

| # | Severity | Bug | Location |
|---|----------|-----|----------|
| 1 | CRITICAL | Cancelled jobs execute to completion — no status checks in loops | `engine/engine.go:157-176`, `233-280` |
| 2 | CRITICAL | No cancellation mechanism — `CancelJob()` missing, no API/UI | `database/database.go`, all routes |
| 3 | CRITICAL | Cron scheduler never stopped — goroutine leak, no graceful shutdown | `engine/scheduler.go:25-32` |
| 4 | HIGH | Duplicate jobs can run simultaneously — no active job check before launch | `engine/routes.go:975-1053`, `page_handlers.go:760-780` |
| 5 | HIGH | No `context.Context` — cannot propagate cancellation or timeouts | All job functions in `engine/engine.go` |
| 6 | MEDIUM | Startup ingestion job not tracked — invisible, can overlap manual triggers | `engine/scheduler.go:21-23` |
| 7 | MEDIUM | Cron schedule fixed at startup — config changes require restart | `engine/scheduler.go:27-29` |
| 8 | MEDIUM | Panic recovery can leave jobs stuck in "running" forever | `engine/engine.go:108-113` |
| 9 | LOW | Orphaned pending/running jobs permanently disable UI buttons | `database/pg_database.go:576-585` |
| 10 | LOW | Manual triggers bypass cron's `SkipIfStillRunning` guard | `engine/scheduler.go:28` vs `engine/routes.go:987` |

### Root Cause

The fundamental issue is that the job system was designed as a **progress reporting layer** (create record, update progress, mark complete) but not as a **job lifecycle manager** (create, run, cancel, timeout, prevent duplicates, shutdown). The goroutines are fire-and-forget with no back-channel for cancellation or coordination.

### Recommended Fixes

1. **Add `context.Context` to all job functions** — propagate from server shutdown signal
2. **Store cron scheduler in `ServerHandler`** — call `c.Stop()` on shutdown
3. **Add `CancelJob()` to Repository** — sets status to cancelled
4. **Check job status in processing loops** — poll DB or use context cancellation
5. **Check `GetActiveJobs()` before creating new jobs** — reject if same type already running
6. **Use tracked job for startup ingestion** — call `ingressJobFuncWithTracking()` instead
7. **Add job timeout** — mark stuck jobs as failed after configurable duration
8. **Add graceful shutdown** — signal context cancellation, wait for goroutines to finish
