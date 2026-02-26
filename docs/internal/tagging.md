# Tagging System

godocs uses a flexible tagging system for organising documents. Tags can be free-form labels, grouped into mutually-exclusive categories, or promoted to stories that group related documents over time.

## Concepts

### Free tags

A tag with no `tag_group` is a **free tag**. A document can have any number of free tags. Use free tags for topics, subjects, or ad-hoc labels:

- `invoice`, `receipt`, `contract`, `letter`
- `medical`, `tax`, `insurance`, `legal`

### Tag groups (one-per-group)

A tag with a non-null `tag_group` belongs to a **group**. A document can have at most **one** tag from each group. This is enforced by a database trigger so the constraint holds for both UI and API operations.

Built-in groups and typical values:

| Group | Values | Purpose |
|-------|--------|---------|
| Person | parent1, parent2, child1, child2, child3, family, business | Who the document relates to |
| Location | home, office, bank, medical, legal, insurance, tax, education | Where it belongs |
| Importance | low, medium, high, critical | Priority / significance |
| Retention | temporary, keep_1_year, keep_3_years, keep_7_years, keep_10_years, keep_permanent | How long to keep |

You can create new groups by setting `tag_group` on a tag via the tag manager or API.

### Stories

A **story** is a group of documents that belong together (e.g. "House Purchase 2025", "Insurance Claim #4821"). Each story is backed by a tag with `tag_group = "Story"`. Adding a document to a story assigns the story's tag.

Stories additionally support:

- **Start / end dates** to define a time range
- **Associated tags** — extra tags automatically applied when a document joins the story
- **Conversion** — any tag can be promoted to a story, and stories can be demoted back to plain tags

### Dimensions (legacy)

Dimensions were the original structured-metadata system (person, location, importance, retention, year). Migration 006 converted dimension values into **grouped tags**, unifying the two concepts. The dimensions tables still exist in the database but grouped tags are the primary mechanism going forward.

The `.tags.json` sidecar format preserves the distinction via the `tag_groups` field for backward compatibility with external tools.

## Using tags in the UI

### Document edit page

Navigate to any document and click **Edit**. The edit page shows:

1. **Current Tags** — tags already on the document, with a delete button on each.
2. **Quick Tags** — the 15 most-used tags across all documents, shown as clickable buttons. Tags already assigned appear greyed out with a tick. Click an unassigned quick tag to add it immediately.
3. **Add Tag** — a dropdown of all remaining tags for less common tagging.

### Tag manager (`/tags`)

The tag manager lists every tag with its usage count. From here you can:

- Create tags (name, colour, optional group)
- Edit tags (rename, recolour, change group/sort order)
- Delete tags (cascades to remove from all documents)
- Convert a tag to a story (or vice versa)

When a tag is renamed, the old name is stored as an **alias** so `.tags.json` sidecar files with the old name still resolve correctly.

## Using tags externally (API)

All tag operations are available via the REST API. External tools (uploaders, importers, scripts) should use these endpoints rather than writing sidecar files directly.

### List tags

```bash
# All tags (sorted by group, sort_order, name)
curl http://localhost:8000/api/tags

# Tags with document counts
curl http://localhost:8000/api/tags/usage

# Distinct group names
curl http://localhost:8000/api/tags/groups
```

### Create a tag

```bash
curl -X POST http://localhost:8000/api/tags \
  -H 'Content-Type: application/json' \
  -d '{"name": "medical", "color": "#e74c3c"}'
```

Optional fields: `description`, `tag_group`, `sort_order`.

### Add / remove tags on a document

```bash
# Add (idempotent — re-adding is a no-op)
curl -X POST "http://localhost:8000/api/documents/$ULID/tags" \
  -H 'Content-Type: application/json' \
  -d '{"tag_id": 5}'

# Remove
curl -X DELETE "http://localhost:8000/api/documents/$ULID/tags/5"
```

Both operations auto-export the updated tag set to the document's `.tags.json` sidecar.

### Typical import workflow

1. Upload a document (`POST /api/document/upload`)
2. Set OCR text (`PUT /api/document/$ULID/ocr`)
3. Set metadata (`PUT /api/document/$ULID/metadata`)
4. Look up the tag IDs you need (`GET /api/tags`)
5. Add each tag (`POST /api/documents/$ULID/tags`)

See `docs/internal/agents.md` for the full external-agent guide.

### Dimensions API (legacy)

```bash
# List dimensions and their values
curl http://localhost:8000/api/dimensions

# Set a dimension on a document
curl -X POST "http://localhost:8000/api/documents/$ULID/dimensions" \
  -H 'Content-Type: application/json' \
  -d '{"dimension_name": "person", "value": "parent1"}'

# Remove a dimension
curl -X DELETE "http://localhost:8000/api/documents/$ULID/dimensions/person"
```

Prefer using grouped tags for new integrations; the dimensions API is retained for backward compatibility.

## .tags.json sidecar format

Each document can have a `.tags.json` sidecar alongside it on disk. The server creates and updates this file automatically; external tools should **not** write it directly.

```json
{
  "tags": ["invoice", "utilities"],
  "tag_groups": {
    "person": "parent1",
    "location": "home",
    "importance": "high",
    "retention": "keep_7_years"
  }
}
```

- `tags` — array of free tag names
- `tag_groups` — map of group name to tag name (one per group)

During ingestion the server reads the sidecar, resolves aliases, creates missing tags, and applies them. After any API change to a document's tags, the sidecar is re-exported.

## Tag aliases

When a tag is renamed (e.g. `invoice` to `tax-invoice`), the old name is stored in the `tag_aliases` table. Alias resolution is transparent:

- `GetTagByName("invoice")` finds the alias and returns the `tax-invoice` tag
- Sidecar files referencing the old name still work on re-import
- Aliases are also exported to the config file for use by external tools (godocs-inbox)

## Data model summary

| Table | Purpose |
|-------|---------|
| `tags` | Tag definitions (name, colour, group, sort order) |
| `document_tags` | Many-to-many junction: document ↔ tag |
| `tag_aliases` | Old names that resolve to current tags |
| `stories` | Story metadata (title, dates, owning tag_id) |
| `story_tags` | Additional tags associated with a story |
| `dimensions` | Legacy dimension definitions |
| `dimension_values` | Legacy dimension value options |
| `document_dimensions` | Legacy document ↔ dimension assignments |
