package database

import (
	"time"

	"github.com/uptrace/bun"
)

// Tag represents a tag that can be applied to documents
// If TagGroup is nil/empty, it's a free tag (multiple allowed per document)
// If TagGroup has a value, only one tag from that group is allowed per document
type Tag struct {
	bun.BaseModel `bun:"table:tags"`
	ID            int       `json:"id" bun:"id,pk,autoincrement"`
	Name          string    `json:"name" bun:"name"`
	Color         string    `json:"color" bun:"color"`
	Description   string    `json:"description,omitempty" bun:"description"`
	TagGroup      *string   `json:"tag_group,omitempty" bun:"tag_group"`
	SortOrder     int       `json:"sort_order" bun:"sort_order"`
	CreatedAt     time.Time `json:"created_at" bun:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" bun:"updated_at"`
}

// DocumentTag represents the many-to-many relationship between documents and tags
type DocumentTag struct {
	bun.BaseModel `bun:"table:document_tags"`
	DocumentID    int       `json:"document_id" bun:"document_id,pk"`
	TagID         int       `json:"tag_id" bun:"tag_id,pk"`
	CreatedAt     time.Time `json:"created_at" bun:"created_at"`
}

// Dimension represents a structured metadata category (e.g., Person, Location)
type Dimension struct {
	bun.BaseModel `bun:"table:dimensions"`
	ID            int       `json:"id" bun:"id,pk,autoincrement"`
	Name          string    `json:"name" bun:"name"`
	DisplayName   string    `json:"display_name" bun:"display_name"`
	Description   string    `json:"description,omitempty" bun:"description"`
	DimensionType string    `json:"dimension_type" bun:"dimension_type"` // 'single' or 'multiple'
	IsRequired    bool      `json:"is_required" bun:"is_required"`
	CreatedAt     time.Time `json:"created_at" bun:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" bun:"updated_at"`
}

// DimensionValue represents an allowed value for a dimension
type DimensionValue struct {
	bun.BaseModel `bun:"table:dimension_values"`
	ID            int       `json:"id" bun:"id,pk,autoincrement"`
	DimensionID   int       `json:"dimension_id" bun:"dimension_id"`
	Value         string    `json:"value" bun:"value"`
	DisplayName   string    `json:"display_name" bun:"display_name"`
	Description   string    `json:"description,omitempty" bun:"description"`
	Color         string    `json:"color" bun:"color"`
	SortOrder     int       `json:"sort_order" bun:"sort_order"`
	CreatedAt     time.Time `json:"created_at" bun:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" bun:"updated_at"`
}

// DocumentDimension represents a dimension value assigned to a document
type DocumentDimension struct {
	bun.BaseModel    `bun:"table:document_dimensions"`
	ID               int       `json:"id" bun:"id,pk,autoincrement"`
	DocumentID       int       `json:"document_id" bun:"document_id"`
	DimensionID      int       `json:"dimension_id" bun:"dimension_id"`
	DimensionValueID int       `json:"dimension_value_id" bun:"dimension_value_id"`
	CreatedAt        time.Time `json:"created_at" bun:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" bun:"updated_at"`
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
