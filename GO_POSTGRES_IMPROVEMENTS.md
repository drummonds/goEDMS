# go-postgres Suggested Improvements

Workarounds discovered during migration from Bun ORM to `database/sql` + `go-postgres`.
Each item, once implemented in go-postgres, would simplify the godocs codebase.

## Resolved

### 1. SERIAL PRIMARY KEY duplication — Fixed in v0.2.1

**Problem:** `SERIAL` is translated to `INTEGER PRIMARY KEY AUTOINCREMENT`, so
`SERIAL PRIMARY KEY` becomes `INTEGER PRIMARY KEY AUTOINCREMENT PRIMARY KEY` — invalid SQLite.

**Resolution:** Fixed in go-postgres v0.2.1. The `serialPK()` helper has been removed
from godocs; all migrations now use `SERIAL PRIMARY KEY` directly.

### 2. DEFAULT CURRENT_TIMESTAMP not wrapped in parentheses — Fixed in v0.3.0

**Problem:** `DEFAULT CURRENT_TIMESTAMP` is translated to
`DEFAULT datetime('now')`, but SQLite requires `DEFAULT (datetime('now'))` (parentheses
around function calls in DEFAULT clauses).

**Resolution:** Fixed in go-postgres v0.3.0. The `DEFAULT NOW()` workaround has been
removed from godocs; all migrations now use `DEFAULT CURRENT_TIMESTAMP` directly.

### 3. time.Time not coerced on scan — Fixed in v0.3.0

**Problem:** SQLite stores timestamps as strings (e.g. `"2026-01-15 10:30:00 +0000 GMT"`).
When scanning via `database/sql`, Go cannot automatically convert these strings to
`time.Time`, causing `unsupported Scan, storing driver.Value type string into type *time.Time`.

**Resolution:** Fixed in go-postgres v0.3.0. The `timeScanner`/`nullTimeScanner` workaround
can now be removed from `pg_scan.go` and all scan functions simplified to scan directly
into `time.Time` fields.

**Files affected:** `database/pg_scan.go` (can be removed),
`database/pg_database.go`, `database/pg_tags.go`, `database/pg_search.go`.

### 4. NULLS FIRST / NULLS LAST in ORDER BY — Fixed in v0.3.0

**Problem:** PostgreSQL supports `ORDER BY col ASC NULLS FIRST` but go-postgres did not
translate this for SQLite.

**Resolution:** Fixed in go-postgres v0.3.0 for table-qualified columns and complex
expressions. The CASE expression workaround in `pg_tags.go` can now be replaced with
standard `NULLS FIRST` syntax.

**Files affected:** `database/pg_tags.go` — `GetAllTags()` and `GetTagsForDocument()`.

### 5. RETURNING clause support — Confirmed in v0.3.0

**Problem:** `INSERT ... RETURNING id` may not work through pglike.

**Resolution:** Confirmed working in go-postgres v0.3.0 (native SQLite 3.35+ support).
The try/fallback pattern in `pg_tags.go` and `pg_search.go` can be simplified to just
use `RETURNING` directly.

**Files affected:** `database/pg_tags.go` (`CreateTag`), `database/pg_search.go`
(`CreateSavedSearch`).

### 6. ALTER TABLE ADD COLUMN IF NOT EXISTS — Fixed in v0.3.0

**Problem:** PostgreSQL supports `ALTER TABLE ADD COLUMN IF NOT EXISTS` but SQLite does
not (the `IF NOT EXISTS` clause is not recognized for `ADD COLUMN`).

**Resolution:** Fixed in go-postgres v0.3.0. Translates to SQLite-compatible syntax.
The error-ignoring workaround in migrations can now be cleaned up.

**Files affected:** `database/pg_migrations.go` — migration 002 and 006.

## Open

No open issues remaining. All workarounds can be removed now that go-postgres v0.3.0
addresses every known gap.

## Summary

| # | Issue | Status | Workaround |
|---|-------|--------|------------|
| 1 | SERIAL PRIMARY KEY duplication | **Resolved** (v0.2.1) | Removed |
| 2 | DEFAULT CURRENT_TIMESTAMP | **Resolved** (v0.3.0) | Removed |
| 3 | time.Time not coerced on scan | **Resolved** (v0.3.0) | Can remove `pg_scan.go` |
| 4 | NULLS FIRST/LAST | **Resolved** (v0.3.0) | Can remove CASE workaround |
| 5 | RETURNING clause | **Resolved** (v0.3.0) | Can remove try/fallback |
| 6 | ALTER TABLE IF NOT EXISTS | **Resolved** (v0.3.0) | Can remove error ignoring |
