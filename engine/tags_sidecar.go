package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/drummonds/godocs/database"
)

// getTagsSidecarPath returns the path to the tags sidecar file for a document
// For example: /path/to/document.pdf -> /path/to/document.tags.json
func getTagsSidecarPath(docPath string) string {
	ext := filepath.Ext(docPath)
	basePath := docPath[:len(docPath)-len(ext)]
	return basePath + ".tags.json"
}

// readTagsSidecar reads tags and dimensions from a sidecar JSON file
// Returns the tags/dimensions struct and an error if the file doesn't exist or is invalid
func readTagsSidecar(docPath string) (*database.DocumentTagsAndDimensions, error) {
	sidecarPath := getTagsSidecarPath(docPath)

	// Check if sidecar file exists
	if _, err := os.Stat(sidecarPath); os.IsNotExist(err) {
		// No sidecar file - this is not an error, just no tags
		return &database.DocumentTagsAndDimensions{
			Tags:       []string{},
			Dimensions: make(map[string]string),
		}, nil
	}

	// Read the file
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tags sidecar file: %w", err)
	}

	// Parse JSON
	var tagData database.DocumentTagsAndDimensions
	if err := json.Unmarshal(data, &tagData); err != nil {
		return nil, fmt.Errorf("failed to parse tags sidecar JSON: %w", err)
	}

	// Initialize empty collections if nil
	if tagData.Tags == nil {
		tagData.Tags = []string{}
	}
	if tagData.Dimensions == nil {
		tagData.Dimensions = make(map[string]string)
	}

	Logger.Info("Read tags from sidecar", "path", sidecarPath, "tags", len(tagData.Tags), "dimensions", len(tagData.Dimensions))
	return &tagData, nil
}

// writeTagsSidecar writes tags and dimensions to a sidecar JSON file
func writeTagsSidecar(docPath string, tagData *database.DocumentTagsAndDimensions) error {
	sidecarPath := getTagsSidecarPath(docPath)

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(sidecarPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for tags sidecar: %w", err)
	}

	// Convert to pretty JSON
	jsonData, err := json.MarshalIndent(tagData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tags to JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(sidecarPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write tags sidecar file: %w", err)
	}

	Logger.Info("Wrote tags sidecar", "path", sidecarPath, "tags", len(tagData.Tags), "dimensions", len(tagData.Dimensions))
	return nil
}

// applyTagsAndDimensionsToDocument applies tags and dimensions from sidecar to a document in the database
func (serverHandler *ServerHandler) applyTagsAndDimensionsToDocument(doc *database.Document, tagData *database.DocumentTagsAndDimensions, db database.Repository) error {
	// Apply tags
	for _, tagName := range tagData.Tags {
		tagName = strings.TrimSpace(tagName)
		if tagName == "" {
			continue
		}

		// Get or create tag
		tag, err := db.GetTagByName(tagName)
		if err != nil {
			Logger.Warn("Failed to get tag", "tag", tagName, "error", err)
			continue
		}

		// Create tag if it doesn't exist
		if tag == nil {
			tag = &database.Tag{
				Name:  tagName,
				Color: "#3498db", // Default blue color
			}
			if err := db.CreateTag(tag); err != nil {
				Logger.Warn("Failed to create tag", "tag", tagName, "error", err)
				continue
			}
			// Reload to get the ID
			tag, err = db.GetTagByName(tagName)
			if err != nil || tag == nil {
				Logger.Warn("Failed to reload created tag", "tag", tagName, "error", err)
				continue
			}
		}

		// Associate tag with document
		if err := db.AddTagToDocument(doc.StormID, tag.ID); err != nil {
			Logger.Warn("Failed to add tag to document", "tag", tagName, "doc", doc.ULID.String(), "error", err)
		}
	}

	// Apply dimensions
	for dimensionName, valueStr := range tagData.Dimensions {
		valueStr = strings.TrimSpace(valueStr)
		if valueStr == "" {
			continue
		}

		// Get dimension
		dimension, err := db.GetDimensionByName(dimensionName)
		if err != nil {
			Logger.Warn("Failed to get dimension", "dimension", dimensionName, "error", err)
			continue
		}
		if dimension == nil {
			Logger.Warn("Dimension not found", "dimension", dimensionName)
			continue
		}

		// Get dimension value
		dimValue, err := db.GetDimensionValueByValue(dimension.ID, valueStr)
		if err != nil {
			Logger.Warn("Failed to get dimension value", "dimension", dimensionName, "value", valueStr, "error", err)
			continue
		}
		if dimValue == nil {
			Logger.Warn("Dimension value not found", "dimension", dimensionName, "value", valueStr)
			continue
		}

		// Set dimension for document
		if err := db.SetDocumentDimension(doc.StormID, dimension.ID, dimValue.ID); err != nil {
			Logger.Warn("Failed to set document dimension", "dimension", dimensionName, "value", valueStr, "doc", doc.ULID.String(), "error", err)
		}
	}

	return nil
}

// exportTagsAndDimensionsForDocument exports tags and dimensions from database to sidecar file
func (serverHandler *ServerHandler) exportTagsAndDimensionsForDocument(doc *database.Document, db database.Repository) error {
	// Get tags for document
	tags, err := db.GetTagsForDocument(doc.StormID)
	if err != nil {
		return fmt.Errorf("failed to get tags for document: %w", err)
	}

	// Get dimensions for document
	dimensions, err := db.GetDocumentDimensions(doc.StormID)
	if err != nil {
		return fmt.Errorf("failed to get dimensions for document: %w", err)
	}

	// Build tag data structure
	tagData := &database.DocumentTagsAndDimensions{
		Tags:       make([]string, len(tags)),
		Dimensions: make(map[string]string),
	}

	// Add tag names
	for i, tag := range tags {
		tagData.Tags[i] = tag.Name
	}

	// Add dimension values
	for dimName, dimValue := range dimensions {
		tagData.Dimensions[dimName] = dimValue.Value
	}

	// Write to sidecar file
	return writeTagsSidecar(doc.Path, tagData)
}
