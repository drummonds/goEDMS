package engine

import (
	"net/http"
	"strconv"

	"github.com/drummonds/godocs/database"
	"github.com/labstack/echo/v4"
)

// ============================================================================
// TAG ENDPOINTS
// ============================================================================

// GetAllTags returns all available tags
// @Summary Get all tags
// @Description Retrieve all tags in the system
// @Tags Tags
// @Accept json
// @Produce json
// @Success 200 {array} database.Tag
// @Failure 500 {object} map[string]interface{}
// @Router /api/tags [get]
func (serverHandler *ServerHandler) GetAllTags(c echo.Context) error {
	tags, err := serverHandler.DB.GetAllTags()
	if err != nil {
		Logger.Error("Failed to get all tags", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to retrieve tags",
		})
	}
	return c.JSON(http.StatusOK, tags)
}

// CreateTag creates a new tag
// @Summary Create a new tag
// @Description Create a new tag
// @Tags Tags
// @Accept json
// @Produce json
// @Param tag body database.Tag true "Tag object"
// @Success 201 {object} database.Tag
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/tags [post]
func (serverHandler *ServerHandler) CreateTag(c echo.Context) error {
	tag := &database.Tag{}
	if err := c.Bind(tag); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid request body",
		})
	}

	if tag.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Tag name is required",
		})
	}

	// Set default color if not provided
	if tag.Color == "" {
		tag.Color = "#3498db"
	}

	if err := serverHandler.DB.CreateTag(tag); err != nil {
		Logger.Error("Failed to create tag", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to create tag",
		})
	}

	// Reload to get the generated ID
	createdTag, err := serverHandler.DB.GetTagByName(tag.Name)
	if err != nil || createdTag == nil {
		Logger.Error("Failed to retrieve created tag", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Tag created but failed to retrieve",
		})
	}

	return c.JSON(http.StatusCreated, createdTag)
}

// UpdateTag updates an existing tag
// @Summary Update a tag
// @Description Update tag properties
// @Tags Tags
// @Accept json
// @Produce json
// @Param id path int true "Tag ID"
// @Param tag body database.Tag true "Tag object"
// @Success 200 {object} database.Tag
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/tags/{id} [put]
func (serverHandler *ServerHandler) UpdateTag(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid tag ID",
		})
	}

	tag := &database.Tag{}
	if err := c.Bind(tag); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid request body",
		})
	}

	tag.ID = id

	if err := serverHandler.DB.UpdateTag(tag); err != nil {
		Logger.Error("Failed to update tag", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to update tag",
		})
	}

	return c.JSON(http.StatusOK, tag)
}

// DeleteTag deletes a tag
// @Summary Delete a tag
// @Description Delete a tag and remove it from all documents
// @Tags Tags
// @Accept json
// @Produce json
// @Param id path int true "Tag ID"
// @Success 204
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/tags/{id} [delete]
func (serverHandler *ServerHandler) DeleteTag(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid tag ID",
		})
	}

	if err := serverHandler.DB.DeleteTag(id); err != nil {
		Logger.Error("Failed to delete tag", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to delete tag",
		})
	}

	return c.NoContent(http.StatusNoContent)
}

// ============================================================================
// DOCUMENT TAG ENDPOINTS
// ============================================================================

// GetDocumentTags returns all tags for a specific document
// @Summary Get document tags
// @Description Get all tags associated with a document
// @Tags Tags
// @Accept json
// @Produce json
// @Param ulid path string true "Document ULID"
// @Success 200 {array} database.Tag
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/documents/{ulid}/tags [get]
func (serverHandler *ServerHandler) GetDocumentTags(c echo.Context) error {
	ulid := c.Param("ulid")

	doc, err := serverHandler.DB.GetDocumentByULID(ulid)
	if err != nil || doc == nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Document not found",
		})
	}

	tags, err := serverHandler.DB.GetTagsForDocument(doc.StormID)
	if err != nil {
		Logger.Error("Failed to get document tags", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to retrieve tags",
		})
	}

	return c.JSON(http.StatusOK, tags)
}

// AddDocumentTag adds a tag to a document
// @Summary Add tag to document
// @Description Associate a tag with a document
// @Tags Tags
// @Accept json
// @Produce json
// @Param ulid path string true "Document ULID"
// @Param tagId body int true "Tag ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/documents/{ulid}/tags [post]
func (serverHandler *ServerHandler) AddDocumentTag(c echo.Context) error {
	ulid := c.Param("ulid")

	doc, err := serverHandler.DB.GetDocumentByULID(ulid)
	if err != nil || doc == nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Document not found",
		})
	}

	var req struct {
		TagID int `json:"tag_id"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid request body",
		})
	}

	if err := serverHandler.DB.AddTagToDocument(doc.StormID, req.TagID); err != nil {
		Logger.Error("Failed to add tag to document", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to add tag",
		})
	}

	// Export tags to sidecar file
	if err := serverHandler.exportTagsAndDimensionsForDocument(doc, serverHandler.DB); err != nil {
		Logger.Warn("Failed to export tags to sidecar", "error", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Tag added successfully",
	})
}

// RemoveDocumentTag removes a tag from a document
// @Summary Remove tag from document
// @Description Remove a tag association from a document
// @Tags Tags
// @Accept json
// @Produce json
// @Param ulid path string true "Document ULID"
// @Param tagId path int true "Tag ID"
// @Success 204
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/documents/{ulid}/tags/{tagId} [delete]
func (serverHandler *ServerHandler) RemoveDocumentTag(c echo.Context) error {
	ulid := c.Param("ulid")
	tagIDStr := c.Param("tagId")

	tagID, err := strconv.Atoi(tagIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid tag ID",
		})
	}

	doc, err := serverHandler.DB.GetDocumentByULID(ulid)
	if err != nil || doc == nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Document not found",
		})
	}

	if err := serverHandler.DB.RemoveTagFromDocument(doc.StormID, tagID); err != nil {
		Logger.Error("Failed to remove tag from document", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to remove tag",
		})
	}

	// Export tags to sidecar file
	if err := serverHandler.exportTagsAndDimensionsForDocument(doc, serverHandler.DB); err != nil {
		Logger.Warn("Failed to export tags to sidecar", "error", err)
	}

	return c.NoContent(http.StatusNoContent)
}

// ============================================================================
// DIMENSION ENDPOINTS
// ============================================================================

// GetAllDimensions returns all dimension definitions with their values
// @Summary Get all dimensions
// @Description Retrieve all dimension types and their possible values
// @Tags Dimensions
// @Accept json
// @Produce json
// @Success 200 {array} database.DimensionWithValues
// @Failure 500 {object} map[string]interface{}
// @Router /api/dimensions [get]
func (serverHandler *ServerHandler) GetAllDimensions(c echo.Context) error {
	dimensions, err := serverHandler.DB.GetAllDimensions()
	if err != nil {
		Logger.Error("Failed to get dimensions", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to retrieve dimensions",
		})
	}

	// Get values for each dimension
	result := make([]database.DimensionWithValues, len(dimensions))
	for i, dim := range dimensions {
		values, err := serverHandler.DB.GetDimensionValues(dim.ID)
		if err != nil {
			Logger.Warn("Failed to get dimension values", "dimension", dim.Name, "error", err)
			values = []database.DimensionValue{}
		}
		result[i] = database.DimensionWithValues{
			Dimension: dim,
			Values:    values,
		}
	}

	return c.JSON(http.StatusOK, result)
}

// GetDocumentDimensions returns dimension values for a specific document
// @Summary Get document dimensions
// @Description Get all dimension values assigned to a document
// @Tags Dimensions
// @Accept json
// @Produce json
// @Param ulid path string true "Document ULID"
// @Success 200 {object} map[string]database.DimensionValue
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/documents/{ulid}/dimensions [get]
func (serverHandler *ServerHandler) GetDocumentDimensions(c echo.Context) error {
	ulid := c.Param("ulid")

	doc, err := serverHandler.DB.GetDocumentByULID(ulid)
	if err != nil || doc == nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Document not found",
		})
	}

	dimensions, err := serverHandler.DB.GetDocumentDimensions(doc.StormID)
	if err != nil {
		Logger.Error("Failed to get document dimensions", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to retrieve dimensions",
		})
	}

	return c.JSON(http.StatusOK, dimensions)
}

// SetDocumentDimension sets a dimension value for a document
// @Summary Set document dimension
// @Description Set or update a dimension value for a document
// @Tags Dimensions
// @Accept json
// @Produce json
// @Param ulid path string true "Document ULID"
// @Param dimension body object true "Dimension assignment"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/documents/{ulid}/dimensions [post]
func (serverHandler *ServerHandler) SetDocumentDimension(c echo.Context) error {
	ulid := c.Param("ulid")

	doc, err := serverHandler.DB.GetDocumentByULID(ulid)
	if err != nil || doc == nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Document not found",
		})
	}

	var req struct {
		DimensionName string `json:"dimension_name"`
		Value         string `json:"value"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid request body",
		})
	}

	// Get dimension
	dimension, err := serverHandler.DB.GetDimensionByName(req.DimensionName)
	if err != nil || dimension == nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid dimension name",
		})
	}

	// Get dimension value
	dimValue, err := serverHandler.DB.GetDimensionValueByValue(dimension.ID, req.Value)
	if err != nil || dimValue == nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid dimension value",
		})
	}

	// Set dimension
	if err := serverHandler.DB.SetDocumentDimension(doc.StormID, dimension.ID, dimValue.ID); err != nil {
		Logger.Error("Failed to set document dimension", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to set dimension",
		})
	}

	// Export to sidecar file
	if err := serverHandler.exportTagsAndDimensionsForDocument(doc, serverHandler.DB); err != nil {
		Logger.Warn("Failed to export dimensions to sidecar", "error", err)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Dimension set successfully",
	})
}

// RemoveDocumentDimension removes a dimension value from a document
// @Summary Remove document dimension
// @Description Remove a dimension value assignment from a document
// @Tags Dimensions
// @Accept json
// @Produce json
// @Param ulid path string true "Document ULID"
// @Param dimensionName path string true "Dimension name"
// @Success 204
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/documents/{ulid}/dimensions/{dimensionName} [delete]
func (serverHandler *ServerHandler) RemoveDocumentDimension(c echo.Context) error {
	ulid := c.Param("ulid")
	dimensionName := c.Param("dimensionName")

	doc, err := serverHandler.DB.GetDocumentByULID(ulid)
	if err != nil || doc == nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Document not found",
		})
	}

	// Get dimension
	dimension, err := serverHandler.DB.GetDimensionByName(dimensionName)
	if err != nil || dimension == nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error": "Invalid dimension name",
		})
	}

	if err := serverHandler.DB.RemoveDocumentDimension(doc.StormID, dimension.ID); err != nil {
		Logger.Error("Failed to remove document dimension", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to remove dimension",
		})
	}

	// Export to sidecar file
	if err := serverHandler.exportTagsAndDimensionsForDocument(doc, serverHandler.DB); err != nil {
		Logger.Warn("Failed to export dimensions to sidecar", "error", err)
	}

	return c.NoContent(http.StatusNoContent)
}
