# Canonical File Naming

Documents on disk use the DB auto-increment ID with semantic suffixes, organized in a tiered directory structure that scales from 10 to ~10^51 files.

## Directory Structure

Each document is placed under a tier letter (A–Z) followed by zero or more 2-digit directory levels. The directory path is derived from a single global leaf index:

```
leaf = (id - 1) / 10
```

The `leaf` value is decomposed into base-100 pairs. The number of pairs determines the tier letter:

| Tier | Depth | ID Range | Leaf Range | Example Path |
|------|-------|----------|------------|--------------|
| A | 0 | 1–10 | 0 | `A/7.orig.pdf` |
| B | 1 | 11–1,000 | 1–99 | `B/04/42.orig.pdf` |
| C | 2 | 1,001–100,000 | 100–9,999 | `C/01/23/1234.orig.pdf` |
| D | 3 | 100,001–10,000,000 | 10,000–999,999 | `D/01/23/45/1234567.orig.pdf` |
| ... | ... | ... | ... | ... |
| Z | 25 | ... | ... | `Z/01/.../99/{id}.orig.pdf` |

Each leaf directory holds ~10 documents (~30–50 files including sidecars). Tier boundaries fall at clean powers: 10, 1000, 100,000, 10,000,000, ...

## File Naming

```
C/01/23/1234.orig.pdf      # Original document
C/01/23/1234.ocr.txt       # OCR/extracted text
C/01/23/1234.thumb.png     # Thumbnail
C/01/23/1234.tags.json     # Tags metadata
```

## Rules

- Root documents: `.pdf`, `.jpg`, `.jpeg`, `.png`, `.tiff`, `.doc`, `.docx`, `.odf`, `.rtf`, `.text`
- `.orig.` unambiguously marks the primary document
- `.ocr.` marks extracted/OCR text
- `.thumb.png` for thumbnails
- `.tags.json` for tag metadata
- DB `Name` field preserves the original filename for display
- `filepath.Ext("1234.orig.pdf")` returns `.pdf` so content-type serving works

## Path Computation

1. **Compute leaf index:** `leaf = (id - 1) / 10`
2. **Decompose into base-100 pairs:** divide `leaf` repeatedly by 100, collecting 2-digit remainders right-to-left
3. **Determine letter:** `letter = 'A' + number_of_pairs` (leaf 0 → A, 1–99 → B, 100–9999 → C, ...)
4. **Assemble path:** `{root}/{letter}/{pair1}/{pair2}/.../{id}.orig.{ext}`

### Worked examples

**ID 7:**
1. leaf = (7-1)/10 = 0
2. No pairs needed (leaf is 0)
3. Letter = A (depth 0)
4. Path: `A/7.orig.pdf`

**ID 42:**
1. leaf = (42-1)/10 = 4
2. One pair: `04`
3. Letter = B (depth 1)
4. Path: `B/04/42.orig.pdf`

Leaf directory `B/04/` contains IDs 41–50 (10 documents).

**ID 1234:**
1. leaf = (1234-1)/10 = 123
2. Two pairs: 123 → `01`, `23`
3. Letter = C (depth 2)
4. Path: `C/01/23/1234.orig.pdf`

Leaf directory `C/01/23/` contains IDs 1231–1240 (10 documents).

## Legacy Structure (L/K/J)

The previous scheme used reverse-alphabet tier letters with the padded ID as both the directory path and filename:

```
L/00/12/34/001234.orig.pdf     # L tier: IDs 1–99,999 (6-digit, 3 levels)
K/01/23/45/67/01234567.orig.pdf  # K tier: IDs 100,000–9,999,999 (8-digit, 4 levels)
J/00/12/34/56/78/0012345678.orig.pdf  # J tier: IDs 10,000,000+ (10-digit, 5 levels)
```

This put exactly 1 document per leaf directory (~4 files with sidecars). The new A–Z scheme groups ~10 documents per leaf (~30–50 files) and scales to 26 tiers.

The clean DB job migrates documents from legacy to canonical paths automatically.

## Key Functions

- `ComputeNestedPath(id, ext, root)` — full canonical path
- `CanonicalDocName(id, ext)` — e.g. `"1234.orig.pdf"`
- `SidecarBasePath(docPath)` — strips `.orig.{ext}` to get sidecar base
- `getOCRPath(docPath)`, `getThumbPath(docPath)`, `getTagsPath(docPath)` — sidecar paths
