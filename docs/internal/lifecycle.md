# Document Lifecycle

Documents in godocs move through three phases: **ingestion**, **active editing**, and **archival**. This document describes the full lifecycle and the archival design.

## Phases

```
 Ingress folder          Document folder              Archive folder
┌─────────────┐    ┌──────────────────────┐    ┌──────────────────────┐
│  New files   │───>│  Active documents    │───>│  Archive pending     │
│              │    │  (view, tag, edit)   │    │  (files + metadata)  │
└─────────────┘    └──────────────────────┘    └──────────────────────┘
   Ingestion            Active phase                      │
   (existing)           (existing)                        │
                                                          v
                                                   External backup
                                                   tool moves files
                                                          │
                                                          v
                                                 ┌──────────────────┐
                                                 │  Archived         │
                                                 │  (metadata only   │
                                                 │   in DB, frozen)  │
                                                 └──────────────────┘
```

### 1. Ingestion (existing)

Files arrive in the ingress folder and are processed in three steps:

1. **Hash & deduplicate** — MD5 hash calculated, checked against DB
2. **Move to canonical path** — file copied to nested document folder (`L/00/12/34/001234.orig.pdf`), hash verified, source deleted
3. **Extract & enrich** — OCR text extracted, thumbnail generated, `.tags.json` applied, search index updated

No changes needed here.

### 2. Active editing (existing)

Documents in the document folder can be:

- Viewed, searched, tagged, assigned to stories
- Rotated (destructive, rehashes)
- Date and metadata edited
- Bulk-edited via multi-select

No changes needed here.

### 3. Archival (new)

Archival removes documents from day-to-day use while preserving all metadata for audit and recovery. It is a two-stage process: **archive pending** then **archived**.

## Archive design

### Archive folder

A new config value `ARCHIVE_PATH` (default: `archive/` sibling of `DOCUMENT_PATH`). The archive folder mirrors the nested directory structure of the document folder:

```
documents/L/00/12/34/001234.orig.pdf      →  archive/L/00/12/34/001234.orig.pdf
documents/L/00/12/34/001234.ocr.txt       →  archive/L/00/12/34/001234.ocr.txt
documents/L/00/12/34/001234.thumb.png     →  archive/L/00/12/34/001234.thumb.png
documents/L/00/12/34/001234.tags.json     →  archive/L/00/12/34/001234.tags.json
                                              archive/L/00/12/34/001234.lifecycle.json  (new)
```

### Lifecycle metadata file

A new `.lifecycle.json` sidecar is written at archive time. This keeps the `.tags.json` file unchanged (frozen) and records archive-specific metadata separately:

```json
{
  "archived_at": "2026-02-25T14:30:00Z",
  "archived_by": "godocs",
  "archive_reason": "user-initiated",
  "original_path": "L/00/12/34/001234.orig.pdf",
  "hash": "d41d8cd98f00b204e9800998ecf8427e",
  "ulid": "01JFXYZ...",
  "db_id": 1234,
  "schema_version": "1"
}
```

This means:
- `.tags.json` is copied as-is (frozen at archive time)
- `.lifecycle.json` records when, why, and the document identity
- An external backup tool can read `.lifecycle.json` to verify integrity (hash) and track provenance

### Archive states

Archival uses two states tracked via a dedicated `archive_status` column on the `documents` table (not a tag — see rationale below):

| State | `archive_status` | Files on disk | Visible in UI | Editable |
|-------|------------------|---------------|---------------|----------|
| Active | `NULL` | document folder | Yes | Yes |
| Archive pending | `'pending'` | archive folder | Only in archive view | No (frozen) |
| Archived | `'archived'` | removed from archive folder (by external tool) | No | No (frozen) |

**Why a column, not a tag?** The "Archive Pending" concept needs to:
- Prevent edits (tags can't enforce this)
- Filter documents from all default queries (tags require subquery exclusion in every query)
- Track a timestamp (`archived_at`)
- Be queryable without joins

However, an **"Archive Pending" system tag** is also created (like the existing "Hide" tag) so the archive state is visible in the tag UI and in `.tags.json` exports. The tag is applied automatically when archival begins and is the mechanism by which users can *select* documents for archival via the existing bulk-edit multi-select.

### Workflow

#### Selecting documents for archival

Uses the existing multi-select system:

1. User visits home page with `?select=1`
2. Selects documents via checkboxes
3. Clicks "Archive Selected" button on the bulk-edit page
4. Confirmation dialog: "Archive N documents? This will move files to the archive folder and freeze metadata."

#### Archive pending stage

When the user confirms:

1. For each document:
   - Set `archive_status = 'pending'`, `archived_at = NOW()` in DB
   - Add the "Archive Pending" system tag
   - Export final `.tags.json` (includes the Archive Pending tag)
   - Write `.lifecycle.json` sidecar
   - Move all files (`.orig.*`, `.ocr.txt`, `.thumb.png`, `.tags.json`, `.lifecycle.json`) to the archive folder, preserving nested structure
   - Update `documents.path` to point to archive location
2. Documents disappear from default views (filtered by `archive_status IS NOT NULL`)
3. Documents are **frozen** — tag/date/metadata edits rejected with "document is archived"

#### Archived stage

An external program (backup tool, rsync script, cloud uploader) is responsible for moving files from the archive folder to long-term storage. Once files are moved:

1. External tool calls: `PUT /api/document/{ulid}/archive-confirm`
2. godocs sets `archive_status = 'archived'`
3. Physical files are now gone from the archive folder
4. DB record retained indefinitely as a metadata-only tombstone

#### Viewing archived documents

- Default queries exclude `archive_status IS NOT NULL`
- A dedicated `/archive` page lists archived documents (metadata only, no file access)
- The `/archive` page shows: name, date, tags, hash, archived_at, archive_status
- No download/view/thumbnail — files are gone

#### Unarchiving (optional, manual)

If files need recovery before the external tool has moved them (i.e. still in archive folder):

1. `PUT /api/document/{ulid}/unarchive`
2. Moves files back from archive folder to document folder
3. Clears `archive_status`, `archived_at`
4. Removes "Archive Pending" tag
5. Fails if `archive_status = 'archived'` (files already gone)

## Database changes

### documents table

```sql
ALTER TABLE documents ADD COLUMN archive_status TEXT;       -- NULL, 'pending', 'archived'
ALTER TABLE documents ADD COLUMN archived_at    TIMESTAMP;  -- when archival began
```

### System tag

Migration creates the "Archive Pending" tag:

```sql
INSERT INTO tags (name, color, description, tag_group, sort_order, created_at, updated_at)
VALUES ('Archive Pending', '#95a5a6', 'Document queued for archival', 'System', 10,
        CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (name) DO NOTHING;
```

## Config changes

```env
ARCHIVE_PATH=archive    # relative or absolute; default: sibling of DOCUMENT_PATH
```

## API endpoints (new)

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/documents/archive` | Archive documents (body: `{"ulids": [...]}`) |
| `PUT` | `/api/document/{ulid}/archive-confirm` | External tool confirms files moved |
| `PUT` | `/api/document/{ulid}/unarchive` | Undo archive-pending (if files still exist) |
| `GET` | `/api/documents/archived` | List archived document metadata |

## UI changes

| Page | Change |
|------|--------|
| Bulk edit | Add "Archive Selected" button |
| Home/search | Filter out `archive_status IS NOT NULL` (like Hide filtering) |
| New `/archive` page | Read-only list of archived documents with metadata |
| Document edit | Reject edits if `archive_status` is set; show "Archived" banner |

## Implementation order

1. Add `archive_status` and `archived_at` columns (migration)
2. Add "Archive Pending" system tag (migration)
3. Add `ARCHIVE_PATH` to config
4. Filter archived documents from default queries
5. Implement archive operation (move files, write `.lifecycle.json`, update DB)
6. Add "Archive Selected" to bulk-edit page
7. Add `/archive` list page
8. Add `archive-confirm` endpoint for external tools
9. Add unarchive endpoint
10. Freeze edits on archived documents

## Interaction with existing features

- **Clean DB**: Skip documents with `archive_status IS NOT NULL` during orphan scanning. Do not delete archive-pending files from the archive folder.
- **Hide tag**: Orthogonal. A document can be hidden (excluded from default view) without being archived. Archival is permanent removal; hiding is temporary suppression.
- **Stories**: Archived documents remain associated with stories in the DB but won't appear in story document lists.
- **Search**: Archived documents excluded from search results by default. The `/archive` page could have its own search.
- **Ingestion**: No interaction. Ingestion only adds new active documents.

## External backup tool contract

The external tool is expected to:

1. Scan the archive folder for `.lifecycle.json` files
2. Read `.lifecycle.json` to get hash, ULID, and document identity
3. Copy/move all sibling files (`.orig.*`, `.ocr.txt`, `.thumb.png`, `.tags.json`, `.lifecycle.json`) to backup storage
4. Verify hash of `.orig.*` matches `.lifecycle.json` hash
5. Call `PUT /api/document/{ulid}/archive-confirm` to mark as archived
6. Delete files from archive folder (or let godocs clean DB do it)

The tool never needs to understand nested paths or canonical naming — it just processes whatever it finds in the archive folder.
