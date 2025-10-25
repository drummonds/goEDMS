# API Endpoint Refactoring - Search Endpoint

## Problem Identified

There was confusion between:
1. **WASM App Route**: `/search` (the search page UI in the browser)
2. **Server API Endpoint**: `/search/*` (the backend API that returns search results)

This created ambiguity because:
- Both used the `/search` path prefix
- It wasn't clear which was the UI route and which was the API endpoint
- Made debugging more difficult
- Violated the convention that all API endpoints should be under `/api/*`

## Solution

Moved the search API endpoint from `/search/*` to `/api/search` to clearly separate:
- **UI Routes**: Everything else (handled by WASM app)
- **API Routes**: `/api/*` (all backend endpoints that return JSON)

## Changes Made

### 1. Server Route Definition (`main.go`)

**Before** (line 158):
```go
e.GET("/search/*", serverHandler.SearchDocuments)

// Admin API routes
e.POST("/api/ingest", serverHandler.RunIngestNow)
```

**After** (lines 159-163):
```go
// API routes
e.GET("/api/search", serverHandler.SearchDocuments)
e.POST("/api/ingest", serverHandler.RunIngestNow)
e.POST("/api/clean", serverHandler.CleanDatabase)
e.GET("/api/about", serverHandler.GetAboutInfo)
```

The search endpoint is now grouped with other API endpoints under the `/api/*` namespace.

### 2. Webapp API Call (`webapp/searchpage.go`)

**Before** (line 88):
```go
searchURL := fmt.Sprintf("/search/?term=%s", encodedTerm)
```

**After** (line 88):
```go
searchURL := fmt.Sprintf("/api/search?term=%s", encodedTerm)
```

The frontend now explicitly calls the `/api/search` endpoint.

### 3. Test Files Updated

Updated all test files to use the new endpoint:

**Files modified**:
- `api_search_test.go` - All test cases updated to `/api/search`
- `api_test.go` - Test setup and assertions updated

**Example** (before):
```go
req := httptest.NewRequest(http.MethodGet, "/search/?term=invoice", nil)
```

**Example** (after):
```go
req := httptest.NewRequest(http.MethodGet, "/api/search?term=invoice", nil)
```

### 4. WASM Binary Rebuilt

Rebuilt `web/app.wasm` with the updated API endpoint call.

## Files Modified

| File | Lines Changed | Description |
|------|---------------|-------------|
| `main.go` | 158-163 | Moved route to `/api/search` and grouped with API routes |
| `webapp/searchpage.go` | 88 | Updated fetch URL to `/api/search` |
| `api_search_test.go` | Multiple | All test URLs updated to `/api/search` |
| `api_test.go` | 57, 187, 198, 220, 501, 569 | Route setup and test URLs updated |
| `web/app.wasm` | Rebuilt | WASM binary with new API endpoint |

## API Endpoint Structure (After Refactoring)

### All API Endpoints Now Under `/api/*`

```
/api/search                        GET    - Search documents by term
/api/ingest                       POST   - Trigger document ingestion
/api/clean                        POST   - Clean database
/api/about                        GET    - Get system information
/api/wordcloud                    GET    - Get word cloud data
/api/wordcloud/recalculate        POST   - Recalculate word cloud
```

### WASM App Routes (Everything Else)

```
/                                 - Home page
/browse                          - Browse documents
/ingest                          - Ingest page
/clean                           - Clean database page
/search                          - Search page UI ✅ This is the WASM route
/wordcloud                       - Word cloud page
/about                           - About page
```

## Clear Separation of Concerns

### Server API Endpoints (`/api/*`)
- Return JSON data
- RESTful operations
- Backend processing
- Examples: `/api/search`, `/api/wordcloud`, `/api/about`

### WASM App Routes (everything else)
- Render UI components
- Handle navigation
- Client-side routing
- Examples: `/search`, `/wordcloud`, `/about`

## Testing Results

All tests pass ✅:

### Search Endpoint Tests
```bash
$ go test -v -run TestSearchEndpoint
=== RUN   TestSearchEndpoint
=== RUN   TestSearchEndpoint/Search_with_valid_term_-_single_word
=== RUN   TestSearchEndpoint/Search_with_valid_term_-_phrase_search
=== RUN   TestSearchEndpoint/Search_with_prefix_matching
=== RUN   TestSearchEndpoint/Search_with_no_results
=== RUN   TestSearchEndpoint/Search_with_empty_term
=== RUN   TestSearchEndpoint/Search_without_term_parameter
=== RUN   TestSearchEndpoint/Search_with_URL_encoded_term
=== RUN   TestSearchEndpoint/Search_with_special_characters
=== RUN   TestSearchEndpoint/Search_results_contain_required_fields
=== RUN   TestSearchEndpoint/Search_case_insensitivity
=== RUN   TestSearchEndpoint/Search_returns_proper_Content-Type
--- PASS: TestSearchEndpoint (0.53s)
PASS
```

### SearchDocuments Tests
```bash
$ go test -v -run TestSearchDocuments
=== RUN   TestSearchDocuments
=== RUN   TestSearchDocuments/Search_-_empty_query_term
=== RUN   TestSearchDocuments/Search_-_with_query_term
=== RUN   TestSearchDocuments/Search_-_phrase_search
--- PASS: TestSearchDocuments (0.51s)
PASS
```

## Usage Examples

### From Browser (WASM App)
User navigates to:
```
http://localhost:8000/search
```
This loads the search page UI. When they search, the UI calls:
```
GET /api/search?term=invoice
```

### From API Client (Direct)
```bash
curl http://localhost:8000/api/search?term=invoice
```

Returns JSON:
```json
{
  "fileSystem": [
    {
      "id": "SearchResults",
      "name": "Search Results",
      "childrenIDs": ["doc1.pdf", "doc2.pdf"],
      ...
    }
  ]
}
```

## Benefits

### 1. **Clear Separation**
- `/api/*` = All backend APIs (JSON)
- Everything else = WASM app routes (UI)

### 2. **Easier Debugging**
- API calls clearly identifiable in network tab
- No confusion about whether `/search` is UI or API

### 3. **Better Documentation**
- API endpoints all in one namespace
- Easy to document: "All APIs are under `/api/*`"

### 4. **Consistent Architecture**
- Follows REST API conventions
- All JSON endpoints grouped together
- Easier for future developers to understand

### 5. **Simpler Testing**
- API tests clearly test `/api/*` endpoints
- UI tests test WASM app routes
- No overlap or confusion

## Migration Notes

### If You Have Bookmarks or Scripts

**Old API calls** (will no longer work):
```bash
curl http://localhost:8000/search/?term=invoice
```

**New API calls** (use these instead):
```bash
curl http://localhost:8000/api/search?term=invoice
```

**WASM app route** (unchanged):
```
http://localhost:8000/search
```
Still works for the UI - just navigate to it in your browser.

## Backward Compatibility

⚠️ **Breaking Change**: The old `/search/*` API endpoint no longer exists.

If you have:
- External scripts calling the search API
- Bookmarked API URLs
- Third-party integrations

You need to update them to use `/api/search` instead of `/search/`.

The WASM app UI route (`/search`) still works normally.

## How to Apply

**Restart your server** to apply the changes:

```bash
# Rebuild (already done)
go build .

# Restart
./goedms
```

**Clear browser cache**:
- Hard refresh: `Ctrl+Shift+R` (Windows/Linux) or `Cmd+Shift+R` (Mac)
- Or use incognito/private window

## Verification

### Test 1: API Endpoint
```bash
curl http://localhost:8000/api/search?term=test
```
Should return JSON search results.

### Test 2: WASM App Route
Open browser:
```
http://localhost:8000/search
```
Should load the search page UI.

### Test 3: Search Functionality
1. Navigate to `http://localhost:8000/search`
2. Enter a search term
3. Click Search
4. Verify results appear

The browser will call `/api/search` in the background.

## Complete API Endpoint Map

Now all server endpoints follow a clear pattern:

### Document Operations
- `GET /home` - Latest documents
- `GET /documents/filesystem` - Document tree
- `GET /document/:id` - Single document
- `DELETE /document/*` - Delete document
- `PATCH /document/move/*` - Move document
- `POST /document/upload` - Upload document

### Folder Operations
- `GET /folder/:folder` - Get folder contents
- `POST /folder/*` - Create folder

### API Operations (under `/api/*`)
- `GET /api/search?term=...` - Search documents ✨ NEW LOCATION
- `POST /api/ingest` - Trigger ingestion
- `POST /api/clean` - Clean database
- `GET /api/about` - System info
- `GET /api/wordcloud` - Word cloud data
- `POST /api/wordcloud/recalculate` - Recalculate word cloud

### Static Assets
- `GET /wasm_exec.js` - WASM runtime
- `GET /app.js` - WASM app loader
- `GET /app.css` - App styles
- `GET /webapp/webapp.css` - Webapp styles
- `GET /webapp/wordcloud.css` - Word cloud styles
- `GET /favicon.ico` - Favicon

### Catch-All
- `ANY /*` - WASM app handler (UI routes)

## Summary

✅ **Moved**: `/search/*` → `/api/search`
✅ **Updated**: Webapp to call new endpoint
✅ **Updated**: All tests to use new endpoint
✅ **Rebuilt**: WASM with new API call
✅ **Tested**: All search tests pass
✅ **Documented**: Clear API structure

**Result**: Clear separation between UI routes and API endpoints. All APIs now under `/api/*` namespace for better organization and clarity! 🎉
