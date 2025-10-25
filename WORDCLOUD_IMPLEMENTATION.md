# Word Cloud Implementation Guide

This document explains how to integrate the word cloud feature into goEDMS.

## Overview

The word cloud feature provides a visual representation of the most frequently occurring words across all documents in the system. It's designed to be computationally efficient by pre-calculating word frequencies and storing them in a database table.

## Architecture

### Database Layer
- **word_frequencies** table: Stores word counts
- **word_cloud_metadata** table: Tracks calculation status
- Incremental updates on document ingestion
- Full recalculation on demand or during database cleaning

### Backend Layer
- Word tokenization and stop word filtering
- PostgreSQL-based frequency storage
- RESTful API endpoints

### Frontend Layer
- Interactive word cloud visualization
- Clickable words that navigate to search
- Font size scaled by frequency
- Color-coded for visual appeal

## Integration Steps

### 1. Run Database Migrations

The migration files have already been created in `database/migrations/`.
They will run automatically on next server start, or you can run them manually:

```bash
# Migrations will auto-run on startup
go run main.go
```

### 2. Update main.go Routes

Add the word cloud routes to your Echo server setup. In `main.go`, find where routes are registered and add:

```go
// Word cloud routes
e.GET("/api/wordcloud", serverHandler.GetWordCloud)
e.POST("/api/wordcloud/recalculate", serverHandler.RecalculateWordCloud)
```

Example location (around line 80-90):

```go
e.GET("/search/*", serverHandler.SearchDocuments)
e.GET("/api/about", serverHandler.GetAboutInfo)
e.POST("/api/ingest", serverHandler.RunIngestNow)
e.POST("/api/clean", serverHandler.CleanDatabase)

// ADD THESE LINES:
e.GET("/api/wordcloud", serverHandler.GetWordCloud)
e.POST("/api/wordcloud/recalculate", serverHandler.RecalculateWordCloud)
```

### 3. Update webapp/app.go

Add the word cloud page to the router. In `webapp/app.go`, update the `renderPage()` function:

```go
func (a *App) renderPage() app.UI {
	switch app.Window().URL().Path {
	case "/":
		return &HomePage{}
	case "/browse":
		return &BrowsePage{}
	case "/ingest":
		return &IngestPage{}
	case "/clean":
		return &CleanPage{}
	case "/search":
		return &SearchPage{}
	case "/about":
		return &AboutPage{}
	case "/wordcloud":  // ADD THIS
		return &WordCloudPage{}
	default:
		return &HomePage{}
	}
}
```

### 4. Update webapp/sidebar.go

Add a menu item for the word cloud. In `webapp/sidebar.go`, add:

```go
app.Li().Body(
	app.A().
		Href("/wordcloud").
		Class("sidebar-link").
		Body(
			app.Span().Text("📊 Word Cloud"),
		),
),
```

### 5. Optional: Update Ingestion to Auto-Calculate

To automatically update word frequencies when documents are ingested, modify the ingestion process.

In `engine/routes.go`, find the `ingressDocument` method and add after document is saved:

```go
// After document is saved to database
err = serverHandler.DB.SaveDocument(newDoc)
if err != nil {
	// ... error handling
}

// ADD THIS: Update word frequencies incrementally
go func(docID string) {
	if err := serverHandler.DB.UpdateWordFrequencies(docID); err != nil {
		Logger.Warn("Failed to update word frequencies", "docID", docID, "error", err)
	}
}(newDoc.ULID.String())
```

### 6. Optional: Integrate with Database Cleaning

In `engine/routes.go`, find the `CleanDatabase` function and add at the end:

```go
// After cleanup is complete, add:
Logger.Info("Recalculating word cloud after database cleanup")
go func() {
	if err := serverHandler.DB.RecalculateAllWordFrequencies(); err != nil {
		Logger.Error("Failed to recalculate word cloud", "error", err)
	}
}()
```

### 7. Add CSS Styling (Optional)

Add CSS for better word cloud visualization in your main stylesheet:

```css
.wordcloud-page {
    padding: 20px;
}

.wordcloud-container {
    background: #f8f9fa;
    border-radius: 8px;
    padding: 20px;
    margin-top: 20px;
}

.wordcloud-metadata {
    display: flex;
    gap: 20px;
    margin-bottom: 20px;
    padding: 15px;
    background: white;
    border-radius: 4px;
}

.wordcloud-metadata p {
    margin: 0;
}

.wordcloud {
    min-height: 400px;
    padding: 30px;
    background: white;
    border-radius: 4px;
}

.word-cloud-item {
    transition: all 0.2s ease;
    font-weight: 600;
}

.word-cloud-item:hover {
    transform: scale(1.1);
    opacity: 0.8;
}

.wordcloud-actions {
    margin-top: 20px;
    display: flex;
    gap: 10px;
}

.refresh-button,
.recalculate-button {
    padding: 10px 20px;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 14px;
}

.refresh-button {
    background: #007bff;
    color: white;
}

.recalculate-button {
    background: #28a745;
    color: white;
}

.refresh-button:hover,
.recalculate-button:hover {
    opacity: 0.9;
}
```

## API Endpoints

### GET /api/wordcloud
Retrieves word cloud data.

**Query Parameters:**
- `limit` (optional): Number of words to return (default: 100, max: 500)

**Response:**
```json
{
  "words": [
    {
      "word": "document",
      "frequency": 245
    },
    ...
  ],
  "metadata": {
    "lastCalculation": "2025-10-25T12:30:00Z",
    "totalDocsProcessed": 150,
    "totalWordsIndexed": 5432,
    "version": 3
  },
  "count": 100
}
```

### POST /api/wordcloud/recalculate
Triggers a full recalculation of word frequencies.

**Response:**
```json
{
  "message": "Word cloud recalculation started",
  "status": "processing"
}
```

## Usage

### Initial Setup

After integration, run:

```bash
# Start the server (migrations run automatically)
go run main.go

# Generate initial word cloud
curl -X POST http://localhost:8000/api/wordcloud/recalculate

# Wait a few seconds, then view
curl http://localhost:8000/api/wordcloud
```

### Via Web Interface

1. Navigate to `/wordcloud` in your browser
2. Click "Generate Word Cloud" if empty
3. Click on any word to search for documents containing it
4. Use "Refresh" to reload data
5. Use "Recalculate" to rebuild from scratch

## Performance Considerations

### Database Impact
- **Initial calculation**: Processes all documents at once (1-5 seconds for 1000 docs)
- **Incremental updates**: Very fast (<10ms per document)
- **Storage**: ~50-200KB for 1000-5000 unique words

### Recommendations
- For <1000 documents: Recalculate on every database clean
- For 1000-10000 documents: Recalculate weekly or monthly
- For >10000 documents: Incremental updates only, recalculate quarterly

### Optimization Options

If performance becomes an issue:

1. **Limit processing to recent documents**:
```sql
-- Only process documents from last 6 months
WHERE ingress_time > NOW() - INTERVAL '6 months'
```

2. **Increase minimum word length** (edit `database/wordcloud.go`):
```go
if len(word) < 4 {  // Changed from 3 to 4
    continue
}
```

3. **Add more stop words** to filter common terms

4. **Use a materialized view** instead of a table for read-heavy workloads

## Troubleshooting

### Word cloud is empty
- Run POST `/api/wordcloud/recalculate`
- Check that documents have `full_text` populated
- Check logs for errors during recalculation

### Word frequencies seem wrong
- Run a full recalculation
- Check if documents were deleted but frequencies not updated

### Performance is slow
- Check the size of `word_frequencies` table: `SELECT COUNT(*) FROM word_frequencies;`
- If >100k rows, consider increasing minimum word length
- Use incremental updates instead of full recalculation

## Future Enhancements

Potential improvements:
- **Stemming**: Group word variants (e.g., "document", "documents", "documenting")
- **Language detection**: Filter by document language
- **Time-based filtering**: Show word cloud for specific date ranges
- **Category filtering**: Word cloud per folder/category
- **Export**: Download word cloud as image/PDF
- **N-grams**: Show common 2-3 word phrases
- **Trending words**: Show words with increasing frequency over time

## Testing

Run the tests:

```bash
# Test word tokenization
go test -v ./database -run TestWordTokenizer

# Test word cloud endpoints
go test -v -run TestWordCloud
```

## Maintenance

### Regular Maintenance
- Monitor table size: `\dt+ word_frequencies` in psql
- Recalculate quarterly or when major document changes occur
- Vacuum the table periodically: `VACUUM ANALYZE word_frequencies;`

### Cleanup
If you want to remove the feature:

```bash
# Run down migration
migrate -path database/migrations -database "postgres://..." down 1
```
