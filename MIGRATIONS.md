# Database Migrations

## Overview

Migrations are Go functions in `database/pg_migrations.go`. They run automatically on startup — no external tools or SQL files needed.

## How It Works

1. A `schema_migrations` table tracks applied versions
2. On startup, `runMigrations()` checks which versions are applied and runs any pending ones
3. Each migration receives `(ctx, db, isPglike)` — the `isPglike` flag lets migrations skip PostgreSQL-specific features (tsvector, triggers, plpgsql) when running on pglike/SQLite

## Current Migrations

| Version | Name | What it does |
|---------|------|-------------|
| 001 | initial_schema | `documents` and `server_config` tables |
| 002 | add_fulltext_search | tsvector column + GIN index + trigger (PG), plain TEXT column (pglike) |
| 003 | add_word_cloud | `word_frequencies` and `word_cloud_metadata` tables |
| 004 | create_jobs_table | Background jobs tracking |
| 005 | add_tagging_system | `tags`, `document_tags`, `dimensions`, `dimension_values`, `document_dimensions` + seed data |
| 006 | unify_tags_dimensions | Migrates dimension values into grouped tags, adds `tag_group`/`sort_order` to tags |
| 007 | create_saved_searches | `saved_searches` table + default Inbox/All searches |
| 008 | add_tagged_saved_search | Adds "Tagged" saved search |
| 009 | add_document_date | `document_date` column on documents |
| 010 | add_stories | `stories` and `story_tags` tables |
| 011 | add_document_metadata | `created_date`, `updated_date`, `author`, `source_url`, `source` columns |
| 012 | add_hide_tag | Inserts "Hide" system tag |
| 013 | add_tag_aliases | `tag_aliases` table for rename tracking |

## Adding a New Migration

1. Add a new function in `database/pg_migrations.go`:

```go
func migrate014MyFeature(ctx context.Context, db *sql.DB, isPglike bool) error {
    _, err := db.ExecContext(ctx, `
        ALTER TABLE documents ADD COLUMN new_field TEXT DEFAULT ''`)
    if err != nil {
        Logger.Warn("Could not add column", "error", err)
    }
    return nil
}
```

2. Register it in the `migrations` slice in `runMigrations()`:

```go
{"014", "my_feature", migrate014MyFeature},
```

3. Test: `go test ./database/... -run TestPglikeDatabase -v`

## Conventions

- Use `IF NOT EXISTS` / `IF EXISTS` for idempotency
- Use `Logger.Warn` for columns that might already exist (non-fatal)
- Use `return fmt.Errorf(...)` for genuinely fatal errors
- Gate PostgreSQL-only SQL (tsvector, triggers, plpgsql) behind `if !isPglike`
- Never modify an existing migration — always add a new one

## Checking Migration Status

```sql
SELECT * FROM schema_migrations ORDER BY version;
```
