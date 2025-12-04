package database

import (
	"context"
	"database/sql"
	"fmt"
)

// ============================================================================
// TAG METHODS
// ============================================================================

// CreateTag creates a new tag
func (b *BunDB) CreateTag(tag *Tag) error {
	ctx := context.Background()
	_, err := b.db.NewInsert().Model(tag).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}
	return nil
}

// GetAllTags returns all tags
func (b *BunDB) GetAllTags() ([]Tag, error) {
	ctx := context.Background()
	var tags []Tag
	err := b.db.NewSelect().
		Model(&tags).
		Order("name ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all tags: %w", err)
	}
	return tags, nil
}

// GetTagByID returns a tag by its ID
func (b *BunDB) GetTagByID(id int) (*Tag, error) {
	ctx := context.Background()
	tag := &Tag{}
	err := b.db.NewSelect().
		Model(tag).
		Where("id = ?", id).
		Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tag by ID: %w", err)
	}
	return tag, nil
}

// GetTagByName returns a tag by its name
func (b *BunDB) GetTagByName(name string) (*Tag, error) {
	ctx := context.Background()
	tag := &Tag{}
	err := b.db.NewSelect().
		Model(tag).
		Where("name = ?", name).
		Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tag by name: %w", err)
	}
	return tag, nil
}

// UpdateTag updates an existing tag
func (b *BunDB) UpdateTag(tag *Tag) error {
	ctx := context.Background()
	_, err := b.db.NewUpdate().
		Model(tag).
		WherePK().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update tag: %w", err)
	}
	return nil
}

// DeleteTag deletes a tag (will also remove all document associations due to CASCADE)
func (b *BunDB) DeleteTag(id int) error {
	ctx := context.Background()
	_, err := b.db.NewDelete().
		Model((*Tag)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}
	return nil
}

// GetTagsForDocument returns all tags associated with a document
func (b *BunDB) GetTagsForDocument(documentID int) ([]Tag, error) {
	ctx := context.Background()
	var tags []Tag

	// Use raw query to avoid any ORM alias issues
	err := b.db.NewRaw(`
		SELECT t.id, t.name, t.color, t.description, t.created_at, t.updated_at
		FROM tags t
		INNER JOIN document_tags dt ON dt.tag_id = t.id
		WHERE dt.document_id = ?
		ORDER BY t.name ASC
	`, documentID).Scan(ctx, &tags)

	if err != nil {
		return nil, fmt.Errorf("failed to get tags for document: %w", err)
	}
	return tags, nil
}

// AddTagToDocument associates a tag with a document
func (b *BunDB) AddTagToDocument(documentID int, tagID int) error {
	ctx := context.Background()
	docTag := &DocumentTag{
		DocumentID: documentID,
		TagID:      tagID,
	}
	_, err := b.db.NewInsert().
		Model(docTag).
		On("CONFLICT (document_id, tag_id) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to add tag to document: %w", err)
	}
	return nil
}

// RemoveTagFromDocument removes a tag association from a document
func (b *BunDB) RemoveTagFromDocument(documentID int, tagID int) error {
	ctx := context.Background()
	_, err := b.db.NewDelete().
		Model((*DocumentTag)(nil)).
		Where("document_id = ? AND tag_id = ?", documentID, tagID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove tag from document: %w", err)
	}
	return nil
}

// GetTagUsageCount returns the number of documents using a specific tag
func (b *BunDB) GetTagUsageCount(tagID int) (int, error) {
	ctx := context.Background()
	count, err := b.db.NewSelect().
		Model((*DocumentTag)(nil)).
		Where("tag_id = ?", tagID).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get tag usage count: %w", err)
	}
	return count, nil
}

// ============================================================================
// DIMENSION METHODS
// ============================================================================

// GetAllDimensions returns all dimension definitions
func (b *BunDB) GetAllDimensions() ([]Dimension, error) {
	ctx := context.Background()
	var dimensions []Dimension
	err := b.db.NewSelect().
		Model(&dimensions).
		Order("name ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all dimensions: %w", err)
	}
	return dimensions, nil
}

// GetDimensionByID returns a dimension by its ID
func (b *BunDB) GetDimensionByID(id int) (*Dimension, error) {
	ctx := context.Background()
	dimension := &Dimension{}
	err := b.db.NewSelect().
		Model(dimension).
		Where("id = ?", id).
		Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get dimension by ID: %w", err)
	}
	return dimension, nil
}

// GetDimensionByName returns a dimension by its name
func (b *BunDB) GetDimensionByName(name string) (*Dimension, error) {
	ctx := context.Background()
	dimension := &Dimension{}
	err := b.db.NewSelect().
		Model(dimension).
		Where("name = ?", name).
		Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get dimension by name: %w", err)
	}
	return dimension, nil
}

// GetDimensionValues returns all possible values for a dimension
func (b *BunDB) GetDimensionValues(dimensionID int) ([]DimensionValue, error) {
	ctx := context.Background()
	var values []DimensionValue
	err := b.db.NewSelect().
		Model(&values).
		Where("dimension_id = ?", dimensionID).
		Order("sort_order ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get dimension values: %w", err)
	}
	return values, nil
}

// GetDimensionValueByValue returns a dimension value by dimension ID and value string
func (b *BunDB) GetDimensionValueByValue(dimensionID int, value string) (*DimensionValue, error) {
	ctx := context.Background()
	dimValue := &DimensionValue{}
	err := b.db.NewSelect().
		Model(dimValue).
		Where("dimension_id = ? AND value = ?", dimensionID, value).
		Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get dimension value: %w", err)
	}
	return dimValue, nil
}

// GetDocumentDimensions returns all dimension values assigned to a document
// Returns a map of dimension_name -> DimensionValue
func (b *BunDB) GetDocumentDimensions(documentID int) (map[string]DimensionValue, error) {
	ctx := context.Background()

	// Query to get dimension values with dimension info
	type dimensionResult struct {
		DimensionValue
		DimensionName string `db:"dimension_name"`
	}

	var results []dimensionResult
	err := b.db.NewSelect().
		ColumnExpr("dv.*, d.name as dimension_name").
		TableExpr("document_dimensions dd").
		Join("INNER JOIN dimension_values dv ON dv.id = dd.dimension_value_id").
		Join("INNER JOIN dimensions d ON d.id = dd.dimension_id").
		Where("dd.document_id = ?", documentID).
		Scan(ctx, &results)

	if err != nil {
		return nil, fmt.Errorf("failed to get document dimensions: %w", err)
	}

	// Convert to map
	dimensions := make(map[string]DimensionValue)
	for _, result := range results {
		dimensions[result.DimensionName] = result.DimensionValue
	}

	return dimensions, nil
}

// SetDocumentDimension sets a dimension value for a document (replaces existing if present)
func (b *BunDB) SetDocumentDimension(documentID int, dimensionID int, dimensionValueID int) error {
	ctx := context.Background()

	docDim := &DocumentDimension{
		DocumentID:       documentID,
		DimensionID:      dimensionID,
		DimensionValueID: dimensionValueID,
	}

	// Use ON CONFLICT to update if exists
	_, err := b.db.NewInsert().
		Model(docDim).
		On("CONFLICT (document_id, dimension_id) DO UPDATE").
		Set("dimension_value_id = EXCLUDED.dimension_value_id").
		Set("updated_at = CURRENT_TIMESTAMP").
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to set document dimension: %w", err)
	}
	return nil
}

// RemoveDocumentDimension removes a dimension value from a document
func (b *BunDB) RemoveDocumentDimension(documentID int, dimensionID int) error {
	ctx := context.Background()

	_, err := b.db.NewDelete().
		Model((*DocumentDimension)(nil)).
		Where("document_id = ? AND dimension_id = ?", documentID, dimensionID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to remove document dimension: %w", err)
	}
	return nil
}
