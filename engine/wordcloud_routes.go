package engine

import (
	"net/http"
	"strconv"

	"codeberg.org/hum3/godocs/database"
	"github.com/labstack/echo/v4"
)

// GetWordCloud returns the top N most frequent words for word cloud visualization
// @Summary Get word cloud data
// @Description Retrieve the top N most frequent words from all documents for word cloud visualization
// @Tags WordCloud
// @Accept json
// @Produce json
// @Param limit query int false "Maximum number of words to return (default: 100, max: 500)"
// @Success 200 {object} map[string]interface{} "Word cloud data with words, metadata, and count"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /wordcloud [get]
func (serverHandler *ServerHandler) GetWordCloud(c echo.Context) error {
	// Get limit parameter (default to 100)
	limit := 100
	if limitParam := c.QueryParam("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 500 {
			limit = l
		}
	}

	// Get top words from database
	words, err := serverHandler.DB.GetTopWords(limit)
	if err != nil {
		Logger.Error("Failed to get word cloud data", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to retrieve word cloud data",
		})
	}

	// Ensure words is never nil (should be handled by DB layer, but safety check)
	if words == nil {
		words = make([]database.WordFrequency, 0)
	}

	// Get metadata
	metadata, err := serverHandler.DB.GetWordCloudMetadata()
	if err != nil {
		Logger.Warn("Failed to get word cloud metadata", "error", err)
		// Return empty metadata instead of nil (zero value for time.Time is 0001-01-01)
		metadata = &database.WordCloudMetadata{
			TotalDocsProcessed: 0,
			TotalWordsIndexed:  0,
			Version:            0,
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"words":    words,
		"metadata": metadata,
		"count":    len(words),
	})
}

// RecalculateWordCloud triggers a full recalculation of word frequencies
// @Summary Recalculate word cloud
// @Description Trigger a full recalculation of word frequencies from all documents
// @Tags WordCloud
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Recalculation started"
// @Router /wordcloud/recalculate [post]
func (serverHandler *ServerHandler) RecalculateWordCloud(c echo.Context) error {
	Logger.Info("Manual word cloud recalculation triggered via API")

	// Run recalculation in a goroutine so we can return immediately
	go func() {
		if err := serverHandler.DB.RecalculateAllWordFrequencies(); err != nil {
			Logger.Error("Word cloud recalculation failed", "error", err)
		} else {
			Logger.Info("Word cloud recalculation completed successfully")
		}
	}()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Word cloud recalculation started",
		"status":  "processing",
	})
}
