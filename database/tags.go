package database

import (
	"time"
)

// Tag represents a free-form tag that can be applied to documents
type Tag struct {
	ID          int       `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Color       string    `json:"color" db:"color"`
	Description string    `json:"description,omitempty" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// DocumentTag represents the many-to-many relationship between documents and tags
type DocumentTag struct {
	DocumentID int       `json:"document_id" db:"document_id"`
	TagID      int       `json:"tag_id" db:"tag_id"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// Dimension represents a structured metadata category (e.g., Person, Location)
type Dimension struct {
	ID            int       `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	DisplayName   string    `json:"display_name" db:"display_name"`
	Description   string    `json:"description,omitempty" db:"description"`
	DimensionType string    `json:"dimension_type" db:"dimension_type"` // 'single' or 'multiple'
	IsRequired    bool      `json:"is_required" db:"is_required"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// DimensionValue represents an allowed value for a dimension
type DimensionValue struct {
	ID          int       `json:"id" db:"id"`
	DimensionID int       `json:"dimension_id" db:"dimension_id"`
	Value       string    `json:"value" db:"value"`
	DisplayName string    `json:"display_name" db:"display_name"`
	Description string    `json:"description,omitempty" db:"description"`
	Color       string    `json:"color" db:"color"`
	SortOrder   int       `json:"sort_order" db:"sort_order"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// DocumentDimension represents a dimension value assigned to a document
type DocumentDimension struct {
	ID               int       `json:"id" db:"id"`
	DocumentID       int       `json:"document_id" db:"document_id"`
	DimensionID      int       `json:"dimension_id" db:"dimension_id"`
	DimensionValueID int       `json:"dimension_value_id" db:"dimension_value_id"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// DocumentTagsAndDimensions is a helper struct for JSON sidecar files
type DocumentTagsAndDimensions struct {
	Tags       []string          `json:"tags"`
	Dimensions map[string]string `json:"dimensions"` // dimension_name -> value
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
	Tags       []Tag                       `json:"tags"`
	Dimensions map[string]DimensionValue   `json:"dimensions"` // dimension_name -> value
}
