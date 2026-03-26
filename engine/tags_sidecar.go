package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/hum3/godocs/database"
)

// readTagsSidecar reads tags and dimensions from a sidecar JSON file
// Returns the tags/dimensions struct and an error if the file doesn't exist or is invalid
func readTagsSidecar(docPath string) (*database.DocumentTagsAndDimensions, error) {
	sidecarPath := getTagsPath(docPath)

	// Check if sidecar file exists
	if _, err := os.Stat(sidecarPath); os.IsNotExist(err) {
		// No sidecar file - this is not an error, just no tags
		return &database.DocumentTagsAndDimensions{
			Tags:      []string{},
			TagGroups: make(map[string]string),
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
	if tagData.TagGroups == nil {
		tagData.TagGroups = make(map[string]string)
	}

	Logger.Info("Read tags from sidecar", "path", sidecarPath, "freeTags", len(tagData.Tags), "tagGroups", len(tagData.TagGroups))
	return &tagData, nil
}

// writeTagsSidecar writes tags and dimensions to a sidecar JSON file
func writeTagsSidecar(docPath string, tagData *database.DocumentTagsAndDimensions) error {
	sidecarPath := getTagsPath(docPath)

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

	Logger.Info("Wrote tags sidecar", "path", sidecarPath, "freeTags", len(tagData.Tags), "groupedTags", len(tagData.TagGroups))
	return nil
}

// applyTagsAndDimensionsToDocument applies tags from sidecar to a document in the database
func (serverHandler *ServerHandler) applyTagsAndDimensionsToDocument(doc *database.Document, tagData *database.DocumentTagsAndDimensions, db database.Repository) error {
	// Apply free tags (tags without a group)
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
		if err := db.AddTagToDocument(doc.ID, tag.ID); err != nil {
			Logger.Warn("Failed to add tag to document", "tag", tagName, "doc", doc.ULID.String(), "error", err)
		}
	}

	// Apply grouped tags (tag_groups: group_name -> tag_name)
	for groupName, tagName := range tagData.TagGroups {
		tagName = strings.TrimSpace(tagName)
		groupName = strings.TrimSpace(groupName)
		if tagName == "" || groupName == "" {
			continue
		}

		// Get the tag by name - it should exist and belong to the correct group
		tag, err := db.GetTagByName(tagName)
		if err != nil {
			Logger.Warn("Failed to get grouped tag", "tag", tagName, "group", groupName, "error", err)
			continue
		}

		if tag == nil {
			// Create the tag with the group
			tag = &database.Tag{
				Name:     tagName,
				Color:    "#3498db",
				TagGroup: &groupName,
			}
			if err := db.CreateTag(tag); err != nil {
				Logger.Warn("Failed to create grouped tag", "tag", tagName, "group", groupName, "error", err)
				continue
			}
			// Reload to get the ID
			tag, err = db.GetTagByName(tagName)
			if err != nil || tag == nil {
				Logger.Warn("Failed to reload created grouped tag", "tag", tagName, "error", err)
				continue
			}
		}

		// Associate tag with document
		if err := db.AddTagToDocument(doc.ID, tag.ID); err != nil {
			Logger.Warn("Failed to add grouped tag to document", "tag", tagName, "group", groupName, "doc", doc.ULID.String(), "error", err)
		}
	}

	return nil
}

// exportTagsAndDimensionsForDocument exports tags and dimensions from database to sidecar file
func (serverHandler *ServerHandler) exportTagsAndDimensionsForDocument(doc *database.Document, db database.Repository) error {
	// Get tags for document
	tags, err := db.GetTagsForDocument(doc.ID)
	if err != nil {
		return fmt.Errorf("failed to get tags for document: %w", err)
	}

	// Build tag data structure - separate free tags from grouped tags
	tagData := &database.DocumentTagsAndDimensions{
		Tags:      []string{},
		TagGroups: make(map[string]string),
	}

	// Categorize tags: free tags vs grouped tags
	for _, tag := range tags {
		if tag.TagGroup != nil && *tag.TagGroup != "" {
			// Grouped tag - store as group_name -> tag_name
			tagData.TagGroups[*tag.TagGroup] = tag.Name
		} else {
			// Free tag - add to tags array
			tagData.Tags = append(tagData.Tags, tag.Name)
		}
	}

	Logger.Info("Exporting tags to sidecar", "path", doc.Path, "freeTags", len(tagData.Tags), "groupedTags", len(tagData.TagGroups))

	// Write to sidecar file
	return writeTagsSidecar(doc.Path, tagData)
}
