# External Agent API Guide

Instructions for automated tools that upload documents and enrich them with metadata.

## Base URL

```
http://localhost:8000
```

## Upload Workflow

### 1. Upload the document

```bash
curl -X POST http://localhost:8000/api/document/upload \
  -F "file=@invoice.pdf"
```

**Response:**
```json
{"path": "/ingress/invoice.pdf", "ulid": "01J...", "name": "invoice.pdf"}
```

The server ingests the file immediately: it hashes it, assigns a DB ID, moves it to canonical storage (`L/00/00/42/000042.orig.pdf`), extracts text if possible, and generates a thumbnail.

**Rejected files:** Sidecar filenames (`.ocr.txt`, `.thumb.png`, `.tags.json`, `.tn_256.png`) are rejected with 400. Use the dedicated endpoints below instead.

### 2. Look up a document by hash

If the upload response didn't include a ULID (e.g. the old code path was used), look it up by MD5 hash:

```bash
HASH=$(md5sum invoice.pdf | cut -d' ' -f1)
curl "http://localhost:8000/api/document/lookup?hash=$HASH"
```

**Response:** Full document JSON including `ulid`, `name`, `path`, `id`.

### 3. Set OCR / extracted text

Use this endpoint to provide or replace extracted text. The server writes the `.ocr.txt` sidecar file and updates the full-text search index. The agent does not need to know the file naming convention.

```bash
curl -X PUT "http://localhost:8000/api/document/$ULID/ocr" \
  -H 'Content-Type: application/json' \
  -d '{"text": "The full extracted text content..."}'
```

**Response:**
```json
{"status": "updated", "ocrPath": "L/00/00/42/000042.ocr.txt"}
```

### 4. Set metadata

All fields are optional. Only non-null fields are updated.

```bash
curl -X PUT "http://localhost:8000/api/document/$ULID/metadata" \
  -H 'Content-Type: application/json' \
  -d '{
    "author": "John Smith",
    "source": "evernote",
    "source_url": "https://www.evernote.com/...",
    "created_date": "2024-03-15T10:30:00Z"
  }'
```

Also generates a thumbnail if one is missing.

### 5. Set document date

```bash
curl -X PUT "http://localhost:8000/api/document/$ULID/date" \
  -H 'Content-Type: application/json' \
  -d '{"date": "2024-03-15"}'
```

### 6. Add tags

First list existing tags to find IDs:

```bash
curl http://localhost:8000/api/tags
```

Then add a tag to the document:

```bash
curl -X POST "http://localhost:8000/api/documents/$ULID/tags" \
  -H 'Content-Type: application/json' \
  -d '{"tag_id": 5}'
```

## Complete Example

```bash
#!/bin/bash
# Upload and enrich a document

FILE="$1"
ULID=$(curl -s -X POST http://localhost:8000/api/document/upload \
  -F "file=@$FILE" | jq -r '.ulid')

echo "Uploaded: $ULID"

# Set OCR text (from an external OCR tool)
TEXT=$(my-ocr-tool "$FILE")
curl -s -X PUT "http://localhost:8000/api/document/$ULID/ocr" \
  -H 'Content-Type: application/json' \
  -d "{\"text\": $(echo "$TEXT" | jq -Rs .)}"

# Set metadata
curl -s -X PUT "http://localhost:8000/api/document/$ULID/metadata" \
  -H 'Content-Type: application/json' \
  -d '{"source": "scanner", "author": "Office"}'

# Add a tag
curl -s -X POST "http://localhost:8000/api/documents/$ULID/tags" \
  -H 'Content-Type: application/json' \
  -d '{"tag_id": 1}'
```

## Endpoint Reference

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/document/upload` | Upload a document file |
| `GET` | `/api/document/lookup?hash=<md5>` | Find document by file hash |
| `PUT` | `/api/document/:id/ocr` | Set OCR text (writes sidecar + DB) |
| `PUT` | `/api/document/:id/text` | Update DB text only (no sidecar) |
| `GET` | `/api/document/:id/text` | Get extracted text |
| `PUT` | `/api/document/:id/metadata` | Set import metadata |
| `PUT` | `/api/document/:id/date` | Set document date |
| `POST` | `/api/documents/:ulid/tags` | Add tag to document |
| `GET` | `/api/tags` | List all tags |
| `GET` | `/api/document/:id/status` | Check thumbnail/text/tag status |
| `POST` | `/api/document/:id/thumbnail/regenerate` | Force thumbnail regeneration |

## Notes

- **Duplicates**: Uploading a file with the same MD5 hash as an existing document is silently skipped. Use `lookup?hash=` to find the existing entry.
- **Idempotent OCR**: Calling `PUT /ocr` multiple times replaces the previous text.
- **Search**: Text set via `/ocr` or `/text` is automatically indexed by PostgreSQL full-text search triggers.
- **File naming**: Agents should never write sidecar files directly to disk. Always use the API endpoints. The server manages canonical naming (`{id}.orig.{ext}`, `{id}.ocr.txt`, etc.).
