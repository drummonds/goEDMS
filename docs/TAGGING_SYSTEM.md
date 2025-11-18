# Tagging System Documentation

## Overview

The godocs tagging system provides comprehensive document organization through:
- **Free-form tags** - Flexible categorization
- **Dimensions** - Structured metadata with predefined values
- **JSON sidecar files** - Portable tag storage alongside documents

## Quick Start

### 1. Apply Migration

The tagging system requires database migration `000005`:

```bash
# Migration will be applied automatically on next startup
# Or manually trigger it through your migration process
```

### 2. Create a Sample Document with Tags

Create a file `example.tags.json` alongside your document:

```json
{
  "tags": ["invoice", "personal", "important"],
  "dimensions": {
    "person": "husband",
    "location": "home",
    "year": "2024",
    "importance": "high",
    "retention": "keep_7_years"
  }
}
```

### 3. Ingest the Document

Place both `example.pdf` and `example.tags.json` in the ingress folder. The tags will be automatically imported.

## Predefined Dimensions

### Person
- `husband`, `wife`, `child1`, `child2`, `child3`, `family`, `business`, `other`

### Location
- `home`, `office`, `bank`, `medical`, `legal`, `insurance`, `tax`, `education`, `other`

### Importance
- `low`, `medium`, `high`, `critical`

### Retention (for future archival)
- `temporary` - < 1 year
- `keep_1_year` - Keep for 1 year
- `keep_3_years` - Keep for 3 years
- `keep_7_years` - Keep for 7 years (tax records)
- `keep_10_years` - Keep for 10 years
- `keep_permanent` - Keep forever (birth certificates, etc.)

### Year
- Values can be any year string (e.g., "2024", "2023")
- Used for archival organization

## API Endpoints

### Tags

**Get all tags:**
```
GET /api/tags
```

**Create tag:**
```
POST /api/tags
Body: {"name": "invoice", "color": "#3498db", "description": "Invoice documents"}
```

**Update tag:**
```
PUT /api/tags/{id}
Body: {"name": "invoice", "color": "#e74c3c"}
```

**Delete tag:**
```
DELETE /api/tags/{id}
```

### Document Tags

**Get document tags:**
```
GET /api/documents/{ulid}/tags
```

**Add tag to document:**
```
POST /api/documents/{ulid}/tags
Body: {"tag_id": 1}
```

**Remove tag from document:**
```
DELETE /api/documents/{ulid}/tags/{tagId}
```

### Dimensions

**Get all dimensions with values:**
```
GET /api/dimensions
```

**Get document dimensions:**
```
GET /api/documents/{ulid}/dimensions
```

**Set document dimension:**
```
POST /api/documents/{ulid}/dimensions
Body: {"dimension_name": "person", "value": "husband"}
```

**Remove document dimension:**
```
DELETE /api/documents/{ulid}/dimensions/{dimensionName}
```

## JSON Sidecar File Format

Sidecar files are named `{document_name}.tags.json`:

**Example: `invoice-2024.pdf` → `invoice-2024.tags.json`**

```json
{
  "tags": [
    "invoice",
    "utilities",
    "electricity"
  ],
  "dimensions": {
    "person": "family",
    "location": "home",
    "year": "2024",
    "importance": "medium",
    "retention": "keep_7_years"
  }
}
```

### Automatic Sidecar Updates

- **During ingestion**: Tags from sidecar files are imported to database
- **After API changes**: Any tag/dimension changes via API automatically update the sidecar file
- **Portability**: Sidecar files travel with documents for backup/migration

## Usage Examples

### Example 1: Medical Document

**File:** `medical-record.pdf`
**Sidecar:** `medical-record.tags.json`

```json
{
  "tags": ["medical", "test-results", "annual-checkup"],
  "dimensions": {
    "person": "wife",
    "location": "medical",
    "year": "2024",
    "importance": "high",
    "retention": "keep_permanent"
  }
}
```

### Example 2: Tax Document

**File:** `tax-return-2024.pdf`
**Sidecar:** `tax-return-2024.tags.json`

```json
{
  "tags": ["tax", "irs", "federal"],
  "dimensions": {
    "person": "family",
    "location": "tax",
    "year": "2024",
    "importance": "critical",
    "retention": "keep_7_years"
  }
}
```

### Example 3: Child's School Document

**File:** `report-card.pdf`
**Sidecar:** `report-card.tags.json`

```json
{
  "tags": ["school", "grades", "semester1"],
  "dimensions": {
    "person": "child1",
    "location": "education",
    "year": "2024",
    "importance": "medium",
    "retention": "keep_3_years"
  }
}
```

## Testing the System

### 1. Test via API

```bash
# Get all dimensions
curl http://localhost:8000/api/dimensions

# Get all tags
curl http://localhost:8000/api/tags

# Create a new tag
curl -X POST http://localhost:8000/api/tags \
  -H "Content-Type: application/json" \
  -d '{"name": "test-tag", "color": "#ff0000"}'
```

### 2. Test via Ingestion

```bash
# Create test document and sidecar
cd /path/to/ingress

# Create a simple text document
echo "Test document" > test.txt

# Create tags sidecar
cat > test.tags.json << 'EOF'
{
  "tags": ["test", "sample"],
  "dimensions": {
    "person": "husband",
    "importance": "low",
    "retention": "temporary"
  }
}
EOF

# Trigger ingestion
curl -X POST http://localhost:8000/api/ingest
```

### 3. Verify Tags Were Applied

```bash
# Get document ULID from the response or database
# Then query its tags
curl http://localhost:8000/api/documents/{ulid}/tags
curl http://localhost:8000/api/documents/{ulid}/dimensions
```

## Future Enhancements

The tagging system is designed to support:

1. **Search by tags** - Find documents by tag combinations
2. **Filter by dimensions** - Filter by person, location, importance, etc.
3. **Auto-archival** - Automatic cleanup based on retention periods
4. **Tag suggestions** - ML-based tag suggestions during upload
5. **Custom dimensions** - User-defined dimension types
6. **Tag hierarchies** - Parent/child tag relationships

## Database Schema

See [000005_add_tagging_system.up.sql](../database/migrations/000005_add_tagging_system.up.sql) for complete schema.

**Key tables:**
- `tags` - Tag definitions
- `document_tags` - Document-tag associations
- `dimensions` - Dimension type definitions
- `dimension_values` - Allowed values per dimension
- `document_dimensions` - Document dimension assignments

## Notes

- Tags are case-sensitive
- Dimension values must match predefined values exactly
- Sidecar files use UTF-8 encoding
- Invalid tags/dimensions in sidecar files are logged as warnings but don't fail ingestion
- All API operations automatically sync with sidecar files
