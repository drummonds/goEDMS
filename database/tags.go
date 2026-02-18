package database

import (
	"time"
)

// Tag represents a tag that can be applied to documents
// If TagGroup is nil/empty, it's a free tag (multiple allowed per document)
// If TagGroup has a value, only one tag from that group is allowed per document
type Tag struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Color       string    `json:"color"`
	Description string    `json:"description,omitempty"`
	TagGroup    *string   `json:"tag_group,omitempty"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TagAliasEntry represents a tag alias with resolved tag name (for config export/import)
type TagAliasEntry struct {
	AliasName string `json:"alias_name"`
	TagName   string `json:"tag_name"`
}

// DocumentTag represents the many-to-many relationship between documents and tags
type DocumentTag struct {
	DocumentID int       `json:"document_id"`
	TagID      int       `json:"tag_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// Dimension represents a structured metadata category (e.g., Person, Location)
type Dimension struct {
	ID            int       `json:"id"`
	Name          string    `json:"name"`
	DisplayName   string    `json:"display_name"`
	Description   string    `json:"description,omitempty"`
	DimensionType string    `json:"dimension_type"` // 'single' or 'multiple'
	IsRequired    bool      `json:"is_required"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// DimensionValue represents an allowed value for a dimension
type DimensionValue struct {
	ID          int       `json:"id"`
	DimensionID int       `json:"dimension_id"`
	Value       string    `json:"value"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description,omitempty"`
	Color       string    `json:"color"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DocumentDimension represents a dimension value assigned to a document
type DocumentDimension struct {
	ID               int       `json:"id"`
	DocumentID       int       `json:"document_id"`
	DimensionID      int       `json:"dimension_id"`
	DimensionValueID int       `json:"dimension_value_id"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// DocumentTagsAndDimensions is a helper struct for JSON sidecar files
type DocumentTagsAndDimensions struct {
	Tags      []string          `json:"tags"`                 // Free tags (no group)
	TagGroups map[string]string `json:"tag_groups,omitempty"` // group_name -> tag_name (one per group)
}

// TagWithCount includes the count of documents using this tag
type TagWithCount struct {
	Tag
	DocumentCount int `json:"document_count" db:"document_count"`
}

// DimensionWithValues includes the dimension and its possible values
type DimensionWithValues struct {
	Dimension
	Values []DimensionValue `json:"values"`
}

// DocumentWithTagsAndDimensions extends Document with its tags and dimensions
type DocumentWithTagsAndDimensions struct {
	Document
	Tags       []Tag                     `json:"tags"`
	Dimensions map[string]DimensionValue `json:"dimensions"` // dimension_name -> value
}
