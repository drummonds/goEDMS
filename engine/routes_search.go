package engine

import (
	"net/http"
	"strconv"

	"github.com/drummonds/godocs/database"
	"github.com/labstack/echo/v4"
)

// ============================================================================
// SAVED SEARCH ENDPOINTS
// ============================================================================

// GetAllSavedSearches returns all saved searches with document counts
// @Summary Get all saved searches
// @Description Retrieve all saved searches with their document counts
// @Tags Searches
// @Accept json
// @Produce json
// @Success 200 {array} database.SavedSearchWithCount
// @Failure 500 {object} map[string]interface{}
// @Router /api/saved-searches [get]
func (serverHandler *ServerHandler) GetAllSavedSearches(c echo.Context) error {
	searches, err := serverHandler.DB.GetAllSavedSearches()
	if err != nil {
		Logger.Error("Failed to get saved searches", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to retrieve saved searches",
		})
	}

	// Add document counts
	result := make([]database.SavedSearchWithCount, len(searches))
	for i, search := range searches {
		parsed := database.ParseSearchQuery(search.Query)
		_, count, err := serverHandler.DB.ExecuteSearch(parsed, 1, 1)
		if err != nil {
			Logger.Warn("Failed to get document count for search", "search", search.Name, "error", err)
			count = 0
		}
		result[i] = database.SavedSearchWithCount{
			SavedSearch:   search,
			DocumentCount: count,
		}
	}

	return c.JSON(http.StatusOK, result)
}

// GetSavedSearch returns a single saved search by ID
// @Summary Get a saved search
// @Description Retrieve a saved search by ID
// @Tags Searches
// @Accept json
// @Produce json
// @Param id path int true "Search ID"
// @Success 200 {object} database.SavedSearch
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/saved-searches/{id} [get]
func (serverHandler *ServerHandler) GetSavedSearch(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid search ID",
		})
	}

	search, err := serverHandler.DB.GetSavedSearchByID(id)
	if err != nil {
		Logger.Error("Failed to get saved search", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to retrieve saved search",
		})
	}
	if search == nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Saved search not found",
		})
	}

	return c.JSON(http.StatusOK, search)
}

// CreateSavedSearch creates a new saved search
// @Summary Create a saved search
// @Description Create a new saved search
// @Tags Searches
// @Accept json
// @Produce json
// @Param search body database.SavedSearch true "Saved search object"
// @Success 201 {object} database.SavedSearch
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/saved-searches [post]
func (serverHandler *ServerHandler) CreateSavedSearch(c echo.Context) error {
	search := &database.SavedSearch{}
	if err := c.Bind(search); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid request body",
		})
	}

	if search.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Search name is required",
		})
	}
	if search.Query == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Search query is required",
		})
	}

	// Set default icon if not provided
	if search.Icon == "" {
		search.Icon = "🔍"
	}

	if err := serverHandler.DB.CreateSavedSearch(search); err != nil {
		Logger.Error("Failed to create saved search", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to create saved search",
		})
	}

	return c.JSON(http.StatusCreated, search)
}

// UpdateSavedSearch updates an existing saved search
// @Summary Update a saved search
// @Description Update saved search properties
// @Tags Searches
// @Accept json
// @Produce json
// @Param id path int true "Search ID"
// @Param search body database.SavedSearch true "Saved search object"
// @Success 200 {object} database.SavedSearch
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/saved-searches/{id} [put]
func (serverHandler *ServerHandler) UpdateSavedSearch(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid search ID",
		})
	}

	// Check if search exists
	existing, err := serverHandler.DB.GetSavedSearchByID(id)
	if err != nil || existing == nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Saved search not found",
		})
	}

	// Don't allow editing system searches
	if existing.IsSystem {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error": "Cannot modify system searches",
		})
	}

	search := &database.SavedSearch{}
	if err := c.Bind(search); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid request body",
		})
	}

	search.ID = id

	if err := serverHandler.DB.UpdateSavedSearch(search); err != nil {
		Logger.Error("Failed to update saved search", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to update saved search",
		})
	}

	return c.JSON(http.StatusOK, search)
}

// DeleteSavedSearch deletes a saved search
// @Summary Delete a saved search
// @Description Delete a saved search by ID
// @Tags Searches
// @Accept json
// @Produce json
// @Param id path int true "Search ID"
// @Success 204
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/saved-searches/{id} [delete]
func (serverHandler *ServerHandler) DeleteSavedSearch(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid search ID",
		})
	}

	// Check if it's a system search
	existing, err := serverHandler.DB.GetSavedSearchByID(id)
	if err == nil && existing != nil && existing.IsSystem {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error": "Cannot delete system searches",
		})
	}

	if err := serverHandler.DB.DeleteSavedSearch(id); err != nil {
		Logger.Error("Failed to delete saved search", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to delete saved search",
		})
	}

	return c.NoContent(http.StatusNoContent)
}

// ============================================================================
// SEARCH EXECUTION ENDPOINTS
// ============================================================================

// SearchResponse represents the response from search execution
type SearchResponse struct {
	Documents   []database.Document `json:"documents"`
	Page        int                 `json:"page"`
	PageSize    int                 `json:"pageSize"`
	TotalCount  int                 `json:"totalCount"`
	TotalPages  int                 `json:"totalPages"`
	HasNext     bool                `json:"hasNext"`
	HasPrevious bool                `json:"hasPrevious"`
	Query       string              `json:"query"`
	SearchName  string              `json:"searchName,omitempty"`
}

// ExecuteSavedSearch executes a saved search by ID
// @Summary Execute a saved search
// @Description Execute a saved search and return paginated results
// @Tags Searches
// @Accept json
// @Produce json
// @Param id path int true "Search ID"
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Success 200 {object} SearchResponse
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/saved-searches/{id}/execute [get]
func (serverHandler *ServerHandler) ExecuteSavedSearch(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid search ID",
		})
	}

	search, err := serverHandler.DB.GetSavedSearchByID(id)
	if err != nil || search == nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Saved search not found",
		})
	}

	// Parse pagination
	page := 1
	pageSize := 20
	if p := c.QueryParam("page"); p != "" {
		if pInt, err := strconv.Atoi(p); err == nil && pInt > 0 {
			page = pInt
		}
	}
	if ps := c.QueryParam("pageSize"); ps != "" {
		if psInt, err := strconv.Atoi(ps); err == nil && psInt > 0 && psInt <= 100 {
			pageSize = psInt
		}
	}

	// Execute search
	parsed := database.ParseSearchQuery(search.Query)
	docs, totalCount, err := serverHandler.DB.ExecuteSearch(parsed, page, pageSize)
	if err != nil {
		Logger.Error("Failed to execute search", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to execute search",
		})
	}

	totalPages := (totalCount + pageSize - 1) / pageSize

	return c.JSON(http.StatusOK, SearchResponse{
		Documents:   docs,
		Page:        page,
		PageSize:    pageSize,
		TotalCount:  totalCount,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
		Query:       search.Query,
		SearchName:  search.Name,
	})
}

// ExecuteAdHocSearch executes an ad-hoc search query
// @Summary Execute an ad-hoc search
// @Description Execute a search query and return paginated results
// @Tags Searches
// @Accept json
// @Produce json
// @Param q query string true "Search query"
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Success 200 {object} SearchResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/search/query [get]
func (serverHandler *ServerHandler) ExecuteAdHocSearch(c echo.Context) error {
	query := c.QueryParam("q")
	if query == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Search query is required",
		})
	}

	// Parse pagination
	page := 1
	pageSize := 20
	if p := c.QueryParam("page"); p != "" {
		if pInt, err := strconv.Atoi(p); err == nil && pInt > 0 {
			page = pInt
		}
	}
	if ps := c.QueryParam("pageSize"); ps != "" {
		if psInt, err := strconv.Atoi(ps); err == nil && psInt > 0 && psInt <= 100 {
			pageSize = psInt
		}
	}

	// Execute search
	parsed := database.ParseSearchQuery(query)
	docs, totalCount, err := serverHandler.DB.ExecuteSearch(parsed, page, pageSize)
	if err != nil {
		Logger.Error("Failed to execute ad-hoc search", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to execute search",
		})
	}

	totalPages := (totalCount + pageSize - 1) / pageSize

	return c.JSON(http.StatusOK, SearchResponse{
		Documents:   docs,
		Page:        page,
		PageSize:    pageSize,
		TotalCount:  totalCount,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
		Query:       query,
	})
}

// GetDocumentsByTag returns documents with a specific tag
// @Summary Get documents by tag
// @Description Get paginated documents that have a specific tag
// @Tags Searches
// @Accept json
// @Produce json
// @Param tagId path int true "Tag ID"
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Success 200 {object} SearchResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/documents/by-tag/{tagId} [get]
func (serverHandler *ServerHandler) GetDocumentsByTag(c echo.Context) error {
	tagIDStr := c.Param("tagId")
	tagID, err := strconv.Atoi(tagIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid tag ID",
		})
	}

	// Get tag name for response
	tag, err := serverHandler.DB.GetTagByID(tagID)
	if err != nil || tag == nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Tag not found",
		})
	}

	// Parse pagination
	page := 1
	pageSize := 20
	if p := c.QueryParam("page"); p != "" {
		if pInt, err := strconv.Atoi(p); err == nil && pInt > 0 {
			page = pInt
		}
	}
	if ps := c.QueryParam("pageSize"); ps != "" {
		if psInt, err := strconv.Atoi(ps); err == nil && psInt > 0 && psInt <= 100 {
			pageSize = psInt
		}
	}

	docs, totalCount, err := serverHandler.DB.GetDocumentsByTag(tagID, page, pageSize)
	if err != nil {
		Logger.Error("Failed to get documents by tag", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to get documents",
		})
	}

	totalPages := (totalCount + pageSize - 1) / pageSize

	return c.JSON(http.StatusOK, SearchResponse{
		Documents:   docs,
		Page:        page,
		PageSize:    pageSize,
		TotalCount:  totalCount,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
		Query:       "#" + tag.Name,
		SearchName:  tag.Name,
	})
}

// GetUntaggedDocuments returns documents without any tags
// @Summary Get untagged documents
// @Description Get paginated documents that have no tags (Inbox)
// @Tags Searches
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(20)
// @Success 200 {object} SearchResponse
// @Failure 500 {object} map[string]interface{}
// @Router /api/documents/untagged [get]
func (serverHandler *ServerHandler) GetUntaggedDocuments(c echo.Context) error {
	// Parse pagination
	page := 1
	pageSize := 20
	if p := c.QueryParam("page"); p != "" {
		if pInt, err := strconv.Atoi(p); err == nil && pInt > 0 {
			page = pInt
		}
	}
	if ps := c.QueryParam("pageSize"); ps != "" {
		if psInt, err := strconv.Atoi(ps); err == nil && psInt > 0 && psInt <= 100 {
			pageSize = psInt
		}
	}

	docs, totalCount, err := serverHandler.DB.GetUntaggedDocuments(page, pageSize)
	if err != nil {
		Logger.Error("Failed to get untagged documents", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to get documents",
		})
	}

	totalPages := (totalCount + pageSize - 1) / pageSize

	return c.JSON(http.StatusOK, SearchResponse{
		Documents:   docs,
		Page:        page,
		PageSize:    pageSize,
		TotalCount:  totalCount,
		TotalPages:  totalPages,
		HasNext:     page < totalPages,
		HasPrevious: page > 1,
		Query:       "!untagged",
		SearchName:  "Inbox",
	})
}
