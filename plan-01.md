# Plan 01: Fix Critical Task Scheduling Bugs

## Bugs Addressed

From `research.md`, the three critical bugs and two high-severity bugs:

| # | Severity | Bug |
|---|----------|-----|
| 1 | CRITICAL | Cancelled jobs execute to completion |
| 2 | CRITICAL | No cancellation mechanism exists |
| 3 | CRITICAL | Cron scheduler never stopped — goroutine leak |
| 4 | HIGH | Duplicate jobs can run simultaneously |
| 5 | HIGH | No `context.Context` in job functions |

Plus medium-severity fixes that fall out naturally:

| 6 | MEDIUM | Startup ingestion job not tracked |
| 7 | MEDIUM | Cron schedule fixed at startup |
| 8 | MEDIUM | Panic recovery leaves jobs stuck |

## Implementation Order

The fixes build on each other, so order matters:

1. Store cron scheduler in `ServerHandler` + graceful shutdown (Bug #3)
2. Add `context.Context` to job functions (Bug #5)
3. Add `CancelJob` method + API endpoint + UI button (Bug #2)
4. Check cancellation in job loops (Bug #1)
5. Prevent duplicate jobs (Bug #4)
6. Track startup ingestion job (Bug #6)
7. Add stuck-job recovery (Bug #8)

---

## Step 1: Store Cron Scheduler in ServerHandler

**Files:** `engine/routes.go`, `engine/scheduler.go`, `main.go`

### 1a. Add scheduler field to ServerHandler

```go
// engine/routes.go
import "github.com/robfig/cron/v3"

type ServerHandler struct {
	DB           database.Repository
	Echo         *echo.Echo
	ServerConfig config.ServerConfig
	cron         *cron.Cron       // stored for graceful shutdown
	cancelJobs   context.CancelFunc // cancels all running jobs
	jobCtx       context.Context    // parent context for all jobs
}
```

### 1b. Rewrite InitializeSchedules to store the cron instance

```go
// engine/scheduler.go
func (serverHandler *ServerHandler) InitializeSchedules(db database.Repository) {
	serverConfig, err := database.FetchConfigFromDB(db)
	if err != nil {
		Logger.Error("Error reading db when initializing", "error", err)
	}

	// Create cancellable context for all jobs
	serverHandler.jobCtx, serverHandler.cancelJobs = context.WithCancel(context.Background())

	// Run ingress job immediately at startup (tracked)
	Logger.Info("Running ingress job at startup")
	go func() {
		job, err := db.CreateJob(database.JobTypeIngestion, "Startup ingestion")
		if err != nil {
			Logger.Error("Failed to create startup ingestion job", "error", err)
			return
		}
		serverHandler.ingressJobFuncWithTracking(serverHandler.jobCtx, serverConfig, db, job.ID)
	}()

	c := cron.New()
	var ingressJob cron.Job
	ingressJob = cron.FuncJob(func() {
		serverHandler.ingressJobFunc(serverHandler.jobCtx, serverConfig, db)
	})
	ingressJob = cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(ingressJob)
	c.AddJob(fmt.Sprintf("@every %dm", serverConfig.IngressInterval), ingressJob)
	Logger.Info("Adding Ingress Job scheduler", "interval_minutes", serverConfig.IngressInterval)
	c.Start()

	serverHandler.cron = c // Store for shutdown
}

// Shutdown stops the cron scheduler and cancels running jobs.
// Call this on server shutdown.
func (serverHandler *ServerHandler) Shutdown() {
	Logger.Info("Shutting down scheduler")
	if serverHandler.cron != nil {
		ctx := serverHandler.cron.Stop() // Stop scheduling new jobs
		<-ctx.Done()                     // Wait for running cron callbacks to finish
	}
	if serverHandler.cancelJobs != nil {
		serverHandler.cancelJobs() // Signal all job goroutines to stop
	}
}
```

### 1c. Add graceful shutdown to main.go

```go
// main.go — after e.Start() or in a signal handler
import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// In the main function, replace the bare e.Start() with:

// Start server in goroutine
go func() {
	if err := e.Start(":" + serverConfig.Port); err != nil && err != http.ErrServerClosed {
		Logger.Error("Server error", "error", err)
	}
}()

// Wait for interrupt signal
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

Logger.Info("Shutting down...")
serverHandler.Shutdown()

ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
if err := e.Shutdown(ctx); err != nil {
	Logger.Error("Server forced shutdown", "error", err)
}
Logger.Info("Server stopped")
```

---

## Step 2: Add context.Context to Job Functions

**Files:** `engine/engine.go`, `engine/ingestion_steps.go`

Thread context through all job functions so cancellation propagates.

### 2a. Update function signatures

```go
// engine/engine.go

func (serverHandler *ServerHandler) ingressJobFunc(
	ctx context.Context,
	serverConfig config.ServerConfig,
	db database.Repository,
) {
	// ... existing body, ctx used in step 2c
}

func (serverHandler *ServerHandler) ingressJobFuncWithTracking(
	ctx context.Context,
	serverConfig config.ServerConfig,
	db database.Repository,
	jobID ulid.ULID,
) {
	// ... existing body, ctx used in step 4
}

func (serverHandler *ServerHandler) cleanupJobFuncWithTracking(
	ctx context.Context,
	db database.Repository,
	jobID ulid.ULID,
) {
	// ... existing body, ctx used in step 4
}
```

### 2b. Update IngestDocumentWithSteps

```go
// engine/ingestion_steps.go
func (serverHandler *ServerHandler) IngestDocumentWithSteps(
	ctx context.Context,
	filePath string,
	db database.Repository,
	jobID ulid.ULID,
	fileNum, totalFiles int,
) error {
	// Check cancellation at entry
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("job cancelled: %w", err)
	}
	// ... rest of existing implementation
}
```

### 2c. Update all callers

Every call site must pass `ctx`. The goroutine launchers use `serverHandler.jobCtx`:

```go
// engine/routes.go — RunIngestNow
go func() {
	serverHandler.ingressJobFuncWithTracking(
		serverHandler.jobCtx, serverHandler.ServerConfig, serverHandler.DB, job.ID,
	)
}()

// engine/routes.go — CleanDatabase
go func() {
	serverHandler.cleanupJobFuncWithTracking(
		serverHandler.jobCtx, serverHandler.DB, job.ID,
	)
}()

// engine/routes.go — RunCleanupAsync, RunIngestionAsync — same pattern
```

---

## Step 3: Add CancelJob Method + API + UI

**Files:** `database/database.go`, `database/pg_database.go`, `database/mem_database.go`, `engine/jobs_routes.go`, `engine/routes.go`, `templates/jobs.html`, `main.go`

### 3a. Add to Repository interface

```go
// database/database.go — add to Repository interface in the Job tracking methods section:
CancelJob(jobID ulid.ULID) error
```

### 3b. Implement in PGDB

```go
// database/pg_database.go
func (p *PGDB) CancelJob(jobID ulid.ULID) error {
	now := time.Now()
	result, err := p.db.ExecContext(context.Background(), `
		UPDATE jobs SET status = $1, message = 'Cancelled by user', updated_at = $2, completed_at = $3
		WHERE id = $4 AND status IN ($5, $6)`,
		JobStatusCancelled, now, now, jobID.String(), JobStatusPending, JobStatusRunning)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("job %s not found or already completed", jobID)
	}
	return nil
}
```

### 3c. Implement in MemDB

```go
// database/mem_database.go
func (m *MemDB) CancelJob(jobID ulid.ULID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, j := range m.jobs {
		if j.ID == jobID && (j.Status == JobStatusPending || j.Status == JobStatusRunning) {
			now := time.Now()
			m.jobs[i].Status = JobStatusCancelled
			m.jobs[i].Message = "Cancelled by user"
			m.jobs[i].UpdatedAt = now
			m.jobs[i].CompletedAt = &now
			return nil
		}
	}
	return fmt.Errorf("job %s not found or already completed", jobID)
}
```

### 3d. Add per-job cancellation tracking

The `CancelJob` DB update alone isn't enough — the goroutine needs a signal. Use a map of per-job cancel functions:

```go
// engine/routes.go — add to ServerHandler
import "sync"

type ServerHandler struct {
	DB           database.Repository
	Echo         *echo.Echo
	ServerConfig config.ServerConfig
	cron         *cron.Cron
	cancelJobs   context.CancelFunc
	jobCtx       context.Context

	jobCancels   map[string]context.CancelFunc // jobID -> cancel func
	jobCancelsMu sync.Mutex
}

// registerJobCancel creates a child context for a specific job and stores its cancel func.
func (sh *ServerHandler) registerJobCancel(jobID ulid.ULID) context.Context {
	sh.jobCancelsMu.Lock()
	defer sh.jobCancelsMu.Unlock()
	if sh.jobCancels == nil {
		sh.jobCancels = make(map[string]context.CancelFunc)
	}
	ctx, cancel := context.WithCancel(sh.jobCtx)
	sh.jobCancels[jobID.String()] = cancel
	return ctx
}

// cancelJob cancels a specific job's context and updates the DB.
func (sh *ServerHandler) cancelJob(jobID ulid.ULID) error {
	sh.jobCancelsMu.Lock()
	cancel, ok := sh.jobCancels[jobID.String()]
	if ok {
		cancel()
		delete(sh.jobCancels, jobID.String())
	}
	sh.jobCancelsMu.Unlock()

	return sh.DB.CancelJob(jobID)
}

// unregisterJob removes the cancel func when a job finishes normally.
func (sh *ServerHandler) unregisterJob(jobID ulid.ULID) {
	sh.jobCancelsMu.Lock()
	delete(sh.jobCancels, jobID.String())
	sh.jobCancelsMu.Unlock()
}
```

### 3e. API endpoint

```go
// engine/jobs_routes.go — add:

// CancelJobHandler cancels a running or pending job
// @Summary Cancel a job
// @Description Cancel a running or pending job by ID
// @Tags Jobs
// @Param id path string true "Job ID (ULID)"
// @Success 200 {object} map[string]interface{} "Job cancelled"
// @Failure 400 {object} map[string]interface{} "Invalid job ID or job not cancellable"
// @Router /jobs/{id}/cancel [post]
func (serverHandler *ServerHandler) CancelJobHandler(c echo.Context) error {
	jobIDStr := c.Param("id")
	jobID, err := ulid.Parse(jobIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid job ID format",
		})
	}

	if err := serverHandler.cancelJob(jobID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Job cancelled",
		"jobId":   jobIDStr,
	})
}
```

### 3f. Register the route

```go
// main.go — in route setup section, alongside existing job routes:
e.POST("/api/jobs/:id/cancel", serverHandler.CancelJobHandler)
```

### 3g. SSR cancel handler

```go
// page_handlers.go — add:
func HandleCancelJob(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		jobIDStr := c.Param("id")
		jobID, err := ulid.Parse(jobIDStr)
		if err != nil {
			Logger.Error("Invalid job ID for cancel", "id", jobIDStr)
			return c.Redirect(http.StatusSeeOther, "/jobs")
		}
		if err := tr.serverHandler.cancelJob(jobID); err != nil {
			Logger.Error("Failed to cancel job", "id", jobIDStr, "error", err)
		}
		return c.Redirect(http.StatusSeeOther, "/jobs")
	}
}
```

Add `serverHandler *engine.ServerHandler` to the `TemplateRenderer` struct (or pass `cancelJob` as a callback, matching the existing pattern for `runCleanupAsync`/`runIngestionAsync`).

### 3h. Register SSR route

```go
// main.go
e.POST("/jobs/:id/cancel", HandleCancelJob(tr))
```

### 3i. UI cancel button

```html
<!-- templates/jobs.html — add cancel button column -->
<thead>
    <tr>
        <th>Type</th>
        <th>Status</th>
        <th>Message</th>
        <th>Progress</th>
        <th>Started</th>
        <th>Completed</th>
        <th></th>  <!-- cancel column -->
    </tr>
</thead>
<tbody>
    {% for job in jobs %}
    <tr>
        <!-- ... existing columns ... -->
        <td>
            {% if job.Status == "running" or job.Status == "pending" %}
            <form method="POST" action="/jobs/{{ job.ID }}/cancel" style="display:inline;">
                <button class="button is-small is-danger is-outlined" type="submit">Cancel</button>
            </form>
            {% endif %}
        </td>
    </tr>
    {% endfor %}
</tbody>
```

---

## Step 4: Check Cancellation in Job Loops

**Files:** `engine/engine.go`, `engine/ingestion_steps.go`

This is the core fix for Bug #1. Every long-running loop checks `ctx.Err()`.

### 4a. Ingestion loop — `ingressJobFuncWithTracking`

```go
// engine/engine.go — inside ingressJobFuncWithTracking, replace the file processing loop:

func (serverHandler *ServerHandler) ingressJobFuncWithTracking(
	ctx context.Context,
	serverConfig config.ServerConfig,
	db database.Repository,
	jobID ulid.ULID,
) {
	defer serverHandler.unregisterJob(jobID)
	defer func() {
		if r := recover(); r != nil {
			Logger.Error("Panic recovered in ingress job", "panic", r, "jobID", jobID)
			db.UpdateJobError(jobID, fmt.Sprintf("Panic: %v", r))
		}
	}()

	// Mark job as running
	if err := db.UpdateJobStatus(jobID, database.JobStatusRunning, "Scanning ingress folder"); err != nil {
		Logger.Error("Failed to update job status", "error", err)
	}

	serverConfig, err := database.FetchConfigFromDB(db)
	if err != nil {
		Logger.Error("Error reading config from database", "error", err)
		db.UpdateJobError(jobID, fmt.Sprintf("Failed to fetch config: %v", err))
		return
	}

	// ... scan for files (unchanged) ...

	for i, filePath := range ingressFiles {
		// === CANCELLATION CHECK ===
		if ctx.Err() != nil {
			Logger.Info("Ingestion job cancelled", "jobID", jobID, "processed", i, "total", totalFiles)
			db.UpdateJobStatus(jobID, database.JobStatusCancelled,
				fmt.Sprintf("Cancelled after processing %d/%d files", i, totalFiles))
			return
		}

		fileName := filepath.Base(filePath)
		Logger.Info("Processing file with step-based ingestion", "file", fileName, "number", i+1, "total", totalFiles)

		err := serverHandler.IngestDocumentWithSteps(ctx, filePath, db, jobID, i, totalFiles)
		// ... error handling unchanged ...
	}

	// ... cleanup, word cloud, complete (unchanged) ...
}
```

### 4b. Cleanup loop — `cleanupJobFuncWithTracking`

Add the same pattern at the top of each major step:

```go
// engine/engine.go — inside cleanupJobFuncWithTracking

// At the top of each step's loop:

// Step 1: Check documents
for i, doc := range documents {
	if ctx.Err() != nil {
		Logger.Info("Cleanup job cancelled", "jobID", jobID, "step", "check documents", "at", i)
		db.UpdateJobStatus(jobID, database.JobStatusCancelled, "Cancelled during document check")
		return
	}
	// ... existing logic ...
}

// Step 2: Orphan rescan
for i, orphanPath := range orphanedFiles {
	if ctx.Err() != nil {
		db.UpdateJobStatus(jobID, database.JobStatusCancelled, "Cancelled during orphan rescan")
		return
	}
	// ... existing logic ...
}

// Steps 3-9: same pattern — check ctx.Err() at the top of each step or loop iteration
```

### 4c. IngestDocumentWithSteps — check between steps

```go
// engine/ingestion_steps.go
func (serverHandler *ServerHandler) IngestDocumentWithSteps(
	ctx context.Context, filePath string, db database.Repository,
	jobID ulid.ULID, fileNum, totalFiles int,
) error {
	fileName := filepath.Base(filePath)
	baseProgress := int((float64(fileNum) / float64(totalFiles)) * 90)

	// Step 1: Hash
	if ctx.Err() != nil {
		return fmt.Errorf("cancelled before step 1")
	}
	// ... step 1 logic ...

	// Step 2: Move
	if ctx.Err() != nil {
		// Rollback: delete the DB record from step 1
		db.DeleteDocument(doc.ULID.String())
		return fmt.Errorf("cancelled before step 2")
	}
	// ... step 2 logic ...

	// Step 3: Extract text
	if ctx.Err() != nil {
		// File is already moved and DB record exists — this is fine,
		// just skip text extraction. Document is still usable.
		Logger.Info("Job cancelled during step 3, document saved without text", "fileName", fileName)
		return nil
	}
	// ... step 3 logic ...
}
```

Note the careful rollback strategy: cancellation between steps 1 and 2 rolls back the DB record. After step 2, the file is already in place, so we keep it.

---

## Step 5: Prevent Duplicate Jobs

**Files:** `engine/routes.go`, `page_handlers.go`

### 5a. Add helper to check for active jobs of a given type

```go
// engine/routes.go

// hasActiveJobOfType returns true if there's already a pending or running job of the given type.
func (sh *ServerHandler) hasActiveJobOfType(jobType database.JobType) bool {
	jobs, err := sh.DB.GetActiveJobs()
	if err != nil {
		Logger.Error("Failed to check active jobs", "error", err)
		return false // fail open — allow the job
	}
	for _, j := range jobs {
		if j.Type == jobType {
			return true
		}
	}
	return false
}
```

### 5b. Guard all job creation points

```go
// engine/routes.go — RunIngestNow
func (serverHandler *ServerHandler) RunIngestNow(c echo.Context) error {
	if serverHandler.hasActiveJobOfType(database.JobTypeIngestion) {
		return c.JSON(http.StatusConflict, map[string]interface{}{
			"error": "An ingestion job is already running",
		})
	}

	job, err := serverHandler.DB.CreateJob(database.JobTypeIngestion, "Starting document ingestion")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to create job"})
	}

	ctx := serverHandler.registerJobCancel(job.ID)
	go func() {
		serverHandler.ingressJobFuncWithTracking(ctx, serverHandler.ServerConfig, serverHandler.DB, job.ID)
	}()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Ingestion started",
		"jobId":   job.ID.String(),
	})
}

// engine/routes.go — CleanDatabase — same pattern with JobTypeCleanup

// engine/routes.go — RunCleanupAsync
func (sh *ServerHandler) RunCleanupAsync() (*database.Job, error) {
	if sh.hasActiveJobOfType(database.JobTypeCleanup) {
		return nil, fmt.Errorf("a cleanup job is already running")
	}
	job, err := sh.DB.CreateJob(database.JobTypeCleanup, "Starting database cleanup")
	if err != nil {
		return nil, err
	}
	ctx := sh.registerJobCancel(job.ID)
	go func() {
		sh.cleanupJobFuncWithTracking(ctx, sh.DB, job.ID)
	}()
	return job, nil
}

// engine/routes.go — RunIngestionAsync — same pattern
```

### 5c. SSR handlers already redirect, just log the error

```go
// page_handlers.go — HandleTriggerClean
func HandleTriggerClean(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		_, err := tr.runCleanupAsync()
		if err != nil {
			Logger.Warn("Failed to trigger cleanup", "error", err)
			// Still redirect — user will see the existing running job on the jobs page
		}
		return c.Redirect(http.StatusSeeOther, "/jobs")
	}
}
```

---

## Step 6: Stuck Job Recovery

**Files:** `engine/scheduler.go` or `main.go`

Jobs that panicked during `UpdateJobError` remain stuck as `running` forever, disabling UI buttons.

### 6a. Add a recovery function

```go
// database/pg_database.go
func (p *PGDB) RecoverStuckJobs(stuckThreshold time.Duration) (int, error) {
	cutoff := time.Now().Add(-stuckThreshold)
	result, err := p.db.ExecContext(context.Background(), `
		UPDATE jobs SET status = $1, error = 'Marked as failed: exceeded timeout',
			updated_at = $2, completed_at = $3
		WHERE status IN ($4, $5) AND updated_at < $6`,
		JobStatusFailed, time.Now(), time.Now(),
		JobStatusPending, JobStatusRunning, cutoff)
	if err != nil {
		return 0, err
	}
	rows, _ := result.RowsAffected()
	return int(rows), nil
}
```

Add to `Repository` interface:

```go
RecoverStuckJobs(stuckThreshold time.Duration) (int, error)
```

### 6b. Run on startup

```go
// engine/scheduler.go — in InitializeSchedules, before starting the cron:

// Recover any jobs stuck from a previous crash
recovered, err := db.RecoverStuckJobs(30 * time.Minute)
if err != nil {
	Logger.Error("Failed to recover stuck jobs", "error", err)
} else if recovered > 0 {
	Logger.Info("Recovered stuck jobs from previous run", "count", recovered)
}
```

---

## Testing Strategy

### Unit tests

```go
// engine/scheduler_test.go
func TestCancelJob_StopsProcessing(t *testing.T) {
	db := database.NewMemDB()
	sh := &ServerHandler{DB: db}
	sh.jobCtx, sh.cancelJobs = context.WithCancel(context.Background())

	job, _ := db.CreateJob(database.JobTypeIngestion, "test")
	ctx := sh.registerJobCancel(job.ID)

	// Cancel immediately
	sh.cancelJob(job.ID)

	// Verify context is cancelled
	if ctx.Err() == nil {
		t.Error("expected context to be cancelled")
	}

	// Verify DB status
	j, _ := db.GetJob(job.ID)
	if j.Status != database.JobStatusCancelled {
		t.Errorf("expected cancelled, got %s", j.Status)
	}
}

func TestDuplicateJobPrevention(t *testing.T) {
	db := database.NewMemDB()
	sh := &ServerHandler{DB: db}
	sh.jobCtx, sh.cancelJobs = context.WithCancel(context.Background())

	// Create a running job
	job, _ := db.CreateJob(database.JobTypeIngestion, "test")
	db.UpdateJobStatus(job.ID, database.JobStatusRunning, "running")

	// Should detect duplicate
	if !sh.hasActiveJobOfType(database.JobTypeIngestion) {
		t.Error("expected active job to be detected")
	}

	// Different type should not be blocked
	if sh.hasActiveJobOfType(database.JobTypeCleanup) {
		t.Error("cleanup should not be blocked by ingestion")
	}
}
```

### Integration test

```go
func TestGracefulShutdown(t *testing.T) {
	db := database.NewMemDB()
	sh := &ServerHandler{DB: db}
	sh.InitializeSchedules(db)

	// Shutdown should not hang
	done := make(chan struct{})
	go func() {
		sh.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown hung")
	}
}
```

## Migration Notes

- No database schema changes required — `JobStatusCancelled` is already in the status enum (stored as text)
- The `jobs` table already has `completed_at` which `CancelJob` populates
- API is additive — new `POST /api/jobs/:id/cancel` endpoint, existing endpoints unchanged
- UI is additive — new cancel button, existing layout unchanged
- All changes are backwards-compatible

## Files Changed Summary

| File | Change |
|------|--------|
| `engine/routes.go` | Add fields to `ServerHandler`, `registerJobCancel`, `cancelJob`, `unregisterJob`, `hasActiveJobOfType`, update `RunIngestNow`, `CleanDatabase`, `RunCleanupAsync`, `RunIngestionAsync` |
| `engine/scheduler.go` | Store cron, add `Shutdown()`, track startup job, recover stuck jobs |
| `engine/engine.go` | Add `ctx context.Context` param to all job funcs, add cancellation checks in loops |
| `engine/ingestion_steps.go` | Add `ctx context.Context` param to `IngestDocumentWithSteps`, check between steps |
| `engine/jobs_routes.go` | Add `CancelJobHandler` |
| `database/database.go` | Add `CancelJob`, `RecoverStuckJobs` to interface |
| `database/pg_database.go` | Implement `CancelJob`, `RecoverStuckJobs` |
| `database/mem_database.go` | Implement `CancelJob`, `RecoverStuckJobs` |
| `main.go` | Register cancel routes, add graceful shutdown signal handler |
| `page_handlers.go` | Add `HandleCancelJob` |
| `templates/jobs.html` | Add cancel button column |
