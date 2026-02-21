# Canonical File Naming

Documents on disk use the DB auto-increment ID with semantic suffixes:

```
L/00/12/34/001234.orig.pdf     # Original document
L/00/12/34/001234.ocr.txt      # OCR/extracted text
L/00/12/34/001234.thumb.png    # Thumbnail
L/00/12/34/001234.tags.json    # Tags metadata
```

## Rules

- ID padding matches tier: 6-digit (L), 8-digit (K), 10-digit (J)
- DB `Name` field preserves the original filename for display
- `.orig.` unambiguously marks the primary document
- `.ocr.` marks extracted/OCR text (replaces legacy `.txt` sidecar)
- `.thumb.png` replaces legacy `.tn_256.png`
- `filepath.Ext("001234.orig.pdf")` returns `.pdf` so content-type serving works

## Migration

The clean DB job (Step 6) automatically migrates documents from legacy naming:

1. Computes expected canonical path via `ComputeNestedPath(id, ext, root)`
2. Skips documents already at canonical paths
3. Renames main file: `oldPath` -> `newPath`
4. Renames legacy sidecars to canonical names:
   - `{base}.txt` -> `{newBase}.ocr.txt`
   - `{base}.tn_256.png` -> `{newBase}.thumb.png`
   - `{base}.tags.json` -> `{newBase}.tags.json`
5. Updates DB path and folder

Both legacy and canonical sidecar patterns are recognized during orphan detection and cleanup for backward compatibility.

## Key Functions

- `ComputeNestedPath(id, ext, root)` — full canonical path
- `CanonicalDocName(id, ext)` — e.g. `"001234.orig.pdf"`
- `SidecarBasePath(docPath)` — strips `.orig.{ext}` to get sidecar base
- `getOCRPath(docPath)`, `getThumbPath(docPath)`, `getTagsPath(docPath)` — sidecar paths
