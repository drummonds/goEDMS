# go-postgres Suggested Improvements

Workarounds discovered during migration from Bun ORM to `database/sql` + `go-postgres`.
Each item, once implemented in go-postgres, would simplify the godocs codebase.

## 1. SERIAL PRIMARY KEY duplication

**Problem:** `SERIAL` is translated to `INTEGER PRIMARY KEY AUTOINCREMENT`, so
`SERIAL PRIMARY KEY` becomes `INTEGER PRIMARY KEY AUTOINCREMENT PRIMARY KEY` — invalid SQLite.

**Current workaround:** A `serialPK()` helper returns `"SERIAL"` for pglike and
`"SERIAL PRIMARY KEY"` for real PostgreSQL, injected via `fmt.Sprintf`.

**Suggested fix:** When go-postgres sees `SERIAL PRIMARY KEY`, strip the redundant
`PRIMARY KEY` during translation since `AUTOINCREMENT` already implies it.

**Files affected:** `database/pg_migrations.go` — all CREATE TABLE statements with SERIAL columns.

## 2. DEFAULT CURRENT_TIMESTAMP not wrapped in parentheses

**Problem:** `DEFAULT CURRENT_TIMESTAMP` is translated to
`DEFAULT datetime('now')`, but SQLite requires `DEFAULT (datetime('now'))` (parentheses
around function calls in DEFAULT clauses).

**Current workaround:** Use `DEFAULT NOW()` instead, which go-postgres wraps correctly.

**Suggested fix:** When translating `CURRENT_TIMESTAMP` in a DEFAULT clause, wrap the
result in parentheses: `DEFAULT (datetime('now'))`.

**Files affected:** `database/pg_migrations.go` — all TIMESTAMP columns use `DEFAULT NOW()`.

## 3. time.Time not coerced on scan

**Problem:** SQLite stores timestamps as strings (e.g. `"2026-01-15 10:30:00 +0000 GMT"`).
When scanning via `database/sql`, Go cannot automatically convert these strings to
`time.Time`, causing `unsupported Scan, storing driver.Value type string into type *time.Time`.

**Current workaround:** Custom `timeScanner` and `nullTimeScanner` types in
`database/pg_scan.go` that implement `sql.Scanner` and parse multiple time string formats.
Every scan function uses these instead of scanning directly into `time.Time`.

**Suggested fix:** Register a custom `time.Time` scanner in the go-postgres driver that
automatically parses timestamp strings returned by SQLite into `time.Time` values. This
would make `database/sql` scan directly into `time.Time` without any custom scanner.

**Files affected:** `database/pg_scan.go` (entire file could be removed),
`database/pg_database.go`, `database/pg_tags.go`, `database/pg_search.go` — all scan
functions could revert to scanning directly into `time.Time` fields.

## 4. NULLS FIRST / NULLS LAST in ORDER BY

**Problem:** PostgreSQL supports `ORDER BY col ASC NULLS FIRST` but go-postgres does not
translate this for SQLite.

**Current workaround:** Use a CASE expression:
```sql
ORDER BY CASE WHEN tag_group IS NULL THEN 0 ELSE 1 END, tag_group ASC
```

**Suggested fix:** Translate `NULLS FIRST` to a CASE expression automatically, or use
SQLite's built-in NULL ordering behavior (NULLs sort first by default in ASC order in
SQLite, so `NULLS FIRST` with ASC could simply be stripped).

**Files affected:** `database/pg_tags.go` — `GetAllTags()` and `GetTagsForDocument()`.

## 5. RETURNING clause support

**Problem:** `INSERT ... RETURNING id` may not work through pglike. PostgreSQL supports
this natively; SQLite added `RETURNING` in 3.35.0 but go-postgres may not pass it through.

**Current workaround:** Try `RETURNING` first, then fall back to a separate
`INSERT` + `SELECT` if it fails.

**Suggested fix:** Either pass `RETURNING` through to SQLite (if version >= 3.35.0) or
translate to an equivalent `SELECT last_insert_rowid()` call.

**Files affected:** `database/pg_tags.go` (`CreateTag`), `database/pg_search.go`
(`CreateSavedSearch`).

## 6. ALTER TABLE ADD COLUMN IF NOT EXISTS

**Problem:** PostgreSQL supports `ALTER TABLE ADD COLUMN IF NOT EXISTS` but SQLite does
not (the `IF NOT EXISTS` clause is not recognized for `ADD COLUMN`).

**Current workaround:** Ignore errors from ALTER TABLE ADD COLUMN (log as warning).

**Suggested fix:** Translate `ADD COLUMN IF NOT EXISTS` to first check
`PRAGMA table_info(table)` for the column, then conditionally run `ADD COLUMN`.

**Files affected:** `database/pg_migrations.go` — migration 002 and 006.

## Summary

| # | Issue | Severity | Workaround complexity |
|---|-------|----------|----------------------|
| 1 | SERIAL PRIMARY KEY duplication | High | `serialPK()` helper + `fmt.Sprintf` in all DDL |
| 2 | DEFAULT CURRENT_TIMESTAMP | Medium | Use `DEFAULT NOW()` everywhere |
| 3 | time.Time not coerced on scan | High | 70-line pg_scan.go + every scan function changed |
| 4 | NULLS FIRST/LAST | Low | CASE expression workaround |
| 5 | RETURNING clause | Medium | Try/fallback pattern |
| 6 | ALTER TABLE IF NOT EXISTS | Low | Ignore errors |
