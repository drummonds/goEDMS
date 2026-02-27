# Upload Deduplication — Client Guide

Instructions for building a client that uploads only new documents to godocs.

## API Reference

### Upload a document

```
POST /api/document/upload
Content-Type: multipart/form-data
Form field: file (the document file)
```

**Responses:**

| Status | Meaning | Body |
|--------|---------|------|
| 201 | Created — new document ingested | `{"ulid": "01J...", "name": "file.pdf", "hash": "abc123...", "id": 42}` |
| 409 | Conflict — duplicate already exists | `{"error": "duplicate document", "hash": "abc123...", "ulid": "01J...", "name": "file.pdf", "id": 42}` |
| 400 | Bad request — sidecar file rejected | `{"error": "cannot upload sidecar files directly; ..."}` |
| 200 | Ingested but ULID lookup failed | `{"path": "/ingress/file.pdf"}` |

The server computes the MD5 hash of the uploaded bytes and checks the database before writing to disk. A 409 response includes the existing document's ULID, so the client can proceed with metadata/tag operations without re-uploading.

Supported file types: `.pdf`, `.jpg`, `.jpeg`, `.png`, `.tiff`, `.doc`, `.docx`, `.odf`, `.rtf`, `.txt`

Rejected sidecar extensions: `.ocr.txt`, `.thumb.png`, `.tags.json`, `.tn_256.png`

### Look up a document by hash

```
GET /api/document/lookup?hash=<md5_hex>
```

**Responses:**

| Status | Meaning | Body |
|--------|---------|------|
| 200 | Found | `{"ulid": "01J...", "name": "file.pdf", "path": "L/00/00/42/000042.orig.pdf", "id": 42, "hash": "abc123..."}` |
| 404 | Not found | — |

### Set OCR text

```
PUT /api/document/:ulid/ocr
Content-Type: application/json
Body: {"text": "extracted text content"}
```

### Set metadata

```
PUT /api/document/:ulid/metadata
Content-Type: application/json
Body: {"author": "...", "source": "scanner", ...}
```

Also auto-generates the thumbnail.

### Add a tag

```
POST /api/documents/:ulid/tags
Content-Type: application/json
Body: {"tag_id": 1}
```

### List all tags

```
GET /api/tags
```

Returns array of `{"id": 1, "name": "Finance", "color": "#3273dc", ...}`.

## Hash Algorithm

MD5, lowercase hex string (32 characters). Example: `d41d8cd98f00b204e9800998ecf8427e`.

Go: `crypto/md5` — the server uses `github.com/drummonds/godocs-hash`.

```go
import "crypto/md5"

func hashFile(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer f.Close()
    h := md5.New()
    if _, err := io.Copy(h, f); err != nil {
        return "", err
    }
    return fmt.Sprintf("%x", h.Sum(nil)), nil
}
```

Shell: `md5sum file.pdf | cut -d' ' -f1`

MD5 throughput is ~1.5 GB/s on modern CPUs. A 50 MB file hashes in ~26 ms. On a Raspberry Pi, expect 50–200 MB/s — still under 1 second for large files.

## Client Upload Strategy

### Simple: upload and handle 409

Upload every file. If the server returns 409, use the ULID from the response body to continue with metadata/tag operations. No client-side hashing needed.

```
for each file:
    resp = POST /api/document/upload with file
    if resp.status == 201:
        ulid = resp.body.ulid       # new document
    elif resp.status == 409:
        ulid = resp.body.ulid       # already exists
    else:
        handle error
    # continue with OCR, metadata, tags using ulid
```

This is simplest but transfers every file over the network.

### Efficient: hash-before-upload

Compute MD5 locally, check via lookup endpoint, skip upload if the document already exists.

```
for each file:
    hash = md5(file)
    resp = GET /api/document/lookup?hash={hash}
    if resp.status == 200:
        ulid = resp.body.ulid       # already on server
    else:
        resp = POST /api/document/upload with file
        ulid = resp.body.ulid       # 201 created
    # continue with OCR, metadata, tags using ulid
```

### Optimal: hash-before-upload + local manifest

For repeated syncs, maintain a local manifest (`path → {size, mtime, md5}`) to skip files that haven't changed since the last sync.

```
for each file:
    stat = os.Stat(file)
    if manifest[path].mtime == stat.mtime && manifest[path].size == stat.size:
        skip                        # unchanged since last sync

    hash = md5(file)
    if manifest[path].hash == hash:
        update manifest mtime, skip # content unchanged despite mtime change

    resp = GET /api/document/lookup?hash={hash}
    if resp.status == 200:
        ulid = resp.body.ulid
    else:
        resp = POST /api/document/upload with file
        ulid = resp.body.ulid

    update manifest {path, size, mtime, hash}
    # continue with OCR, metadata, tags using ulid
```

## Complete Upload Flow (Go pseudocode)

```go
func uploadDocument(client *http.Client, baseURL, filePath string) (string, error) {
    // 1. Hash locally
    hash, err := hashFile(filePath)
    if err != nil {
        return "", err
    }

    // 2. Check if already on server
    resp, err := client.Get(baseURL + "/api/document/lookup?hash=" + hash)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    if resp.StatusCode == 200 {
        var doc struct{ ULID string `json:"ulid"` }
        json.NewDecoder(resp.Body).Decode(&doc)
        return doc.ULID, nil // already exists
    }

    // 3. Upload
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    part, _ := writer.CreateFormFile("file", filepath.Base(filePath))
    f, _ := os.Open(filePath)
    io.Copy(part, f)
    f.Close()
    writer.Close()

    resp, err = client.Post(baseURL+"/api/document/upload", writer.FormDataContentType(), body)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var result struct {
        ULID  string `json:"ulid"`
        Error string `json:"error"`
    }
    json.NewDecoder(resp.Body).Decode(&result)

    switch resp.StatusCode {
    case 201:
        return result.ULID, nil // new document
    case 409:
        return result.ULID, nil // race condition: duplicate appeared between lookup and upload
    default:
        return "", fmt.Errorf("upload failed: %d %s", resp.StatusCode, result.Error)
    }
}
```

## Approaches Considered and Rejected

**Filename + filesize pre-filter** — Not reliable. Same content can have different names; different content can share names.

**Streaming hash during upload** — Wastes bandwidth on duplicates. Hash-before-upload avoids the transfer entirely.

**Partial hashing (first N bytes)** — Not collision-safe for scanned documents with identical headers. Full MD5 is already milliseconds.
