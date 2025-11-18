package database

import (
	"database/sql"
	"fmt"
)

// ============================================================================
// TAG METHODS - PostgreSQL Implementation
// ============================================================================

// CreateTag creates a new tag
func (p *PostgresDB) CreateTag(tag *Tag) error {
	query := `INSERT INTO tags (name, color, description) VALUES ($1, $2, $3) RETURNING id, created_at, updated_at`
	err := p.db.QueryRow(query, tag.Name, tag.Color, tag.Description).Scan(&tag.ID, &tag.CreatedAt, &tag.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create tag: %w", err)
	}
	return nil
}

// GetAllTags returns all tags
func (p *PostgresDB) GetAllTags() ([]Tag, error) {
	query := `SELECT id, name, color, description, created_at, updated_at FROM tags ORDER BY name ASC`
	rows, err := p.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		err := rows.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.Description, &tag.CreatedAt, &tag.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// GetTagByID returns a tag by its ID
func (p *PostgresDB) GetTagByID(id int) (*Tag, error) {
	query := `SELECT id, name, color, description, created_at, updated_at FROM tags WHERE id = $1`
	tag := &Tag{}
	err := p.db.QueryRow(query, id).Scan(&tag.ID, &tag.Name, &tag.Color, &tag.Description, &tag.CreatedAt, &tag.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tag: %w", err)
	}
	return tag, nil
}

// GetTagByName returns a tag by its name
func (p *PostgresDB) GetTagByName(name string) (*Tag, error) {
	query := `SELECT id, name, color, description, created_at, updated_at FROM tags WHERE name = $1`
	tag := &Tag{}
	err := p.db.QueryRow(query, name).Scan(&tag.ID, &tag.Name, &tag.Color, &tag.Description, &tag.CreatedAt, &tag.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tag: %w", err)
	}
	return tag, nil
}

// UpdateTag updates an existing tag
func (p *PostgresDB) UpdateTag(tag *Tag) error {
	query := `UPDATE tags SET name = $1, color = $2, description = $3 WHERE id = $4`
	_, err := p.db.Exec(query, tag.Name, tag.Color, tag.Description, tag.ID)
	if err != nil {
		return fmt.Errorf("failed to update tag: %w", err)
	}
	return nil
}

// DeleteTag deletes a tag
func (p *PostgresDB) DeleteTag(id int) error {
	query := `DELETE FROM tags WHERE id = $1`
	_, err := p.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}
	return nil
}

// GetTagsForDocument returns all tags associated with a document
func (p *PostgresDB) GetTagsForDocument(documentID int) ([]Tag, error) {
	query := `
		SELECT t.id, t.name, t.color, t.description, t.created_at, t.updated_at
		FROM tags t
		INNER JOIN document_tags dt ON dt.tag_id = t.id
		WHERE dt.document_id = $1
		ORDER BY t.name ASC
	`
	rows, err := p.db.Query(query, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query document tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		err := rows.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.Description, &tag.CreatedAt, &tag.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// AddTagToDocument associates a tag with a document
func (p *PostgresDB) AddTagToDocument(documentID int, tagID int) error {
	query := `INSERT INTO document_tags (document_id, tag_id) VALUES ($1, $2) ON CONFLICT (document_id, tag_id) DO NOTHING`
	_, err := p.db.Exec(query, documentID, tagID)
	if err != nil {
		return fmt.Errorf("failed to add tag to document: %w", err)
	}
	return nil
}

// RemoveTagFromDocument removes a tag association from a document
func (p *PostgresDB) RemoveTagFromDocument(documentID int, tagID int) error {
	query := `DELETE FROM document_tags WHERE document_id = $1 AND tag_id = $2`
	_, err := p.db.Exec(query, documentID, tagID)
	if err != nil {
		return fmt.Errorf("failed to remove tag from document: %w", err)
	}
	return nil
}

// ============================================================================
// DIMENSION METHODS - PostgreSQL Implementation
// ============================================================================

// GetAllDimensions returns all dimension definitions
func (p *PostgresDB) GetAllDimensions() ([]Dimension, error) {
	query := `SELECT id, name, display_name, description, dimension_type, is_required, created_at, updated_at FROM dimensions ORDER BY name ASC`
	rows, err := p.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query dimensions: %w", err)
	}
	defer rows.Close()

	var dimensions []Dimension
	for rows.Next() {
		var dim Dimension
		err := rows.Scan(&dim.ID, &dim.Name, &dim.DisplayName, &dim.Description, &dim.DimensionType, &dim.IsRequired, &dim.CreatedAt, &dim.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan dimension: %w", err)
		}
		dimensions = append(dimensions, dim)
	}
	return dimensions, rows.Err()
}

// GetDimensionByID returns a dimension by its ID
func (p *PostgresDB) GetDimensionByID(id int) (*Dimension, error) {
	query := `SELECT id, name, display_name, description, dimension_type, is_required, created_at, updated_at FROM dimensions WHERE id = $1`
	dim := &Dimension{}
	err := p.db.QueryRow(query, id).Scan(&dim.ID, &dim.Name, &dim.DisplayName, &dim.Description, &dim.DimensionType, &dim.IsRequired, &dim.CreatedAt, &dim.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get dimension: %w", err)
	}
	return dim, nil
}

// GetDimensionByName returns a dimension by its name
func (p *PostgresDB) GetDimensionByName(name string) (*Dimension, error) {
	query := `SELECT id, name, display_name, description, dimension_type, is_required, created_at, updated_at FROM dimensions WHERE name = $1`
	dim := &Dimension{}
	err := p.db.QueryRow(query, name).Scan(&dim.ID, &dim.Name, &dim.DisplayName, &dim.Description, &dim.DimensionType, &dim.IsRequired, &dim.CreatedAt, &dim.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get dimension: %w", err)
	}
	return dim, nil
}

// GetDimensionValues returns all possible values for a dimension
func (p *PostgresDB) GetDimensionValues(dimensionID int) ([]DimensionValue, error) {
	query := `SELECT id, dimension_id, value, display_name, description, color, sort_order, created_at, updated_at FROM dimension_values WHERE dimension_id = $1 ORDER BY sort_order ASC`
	rows, err := p.db.Query(query, dimensionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query dimension values: %w", err)
	}
	defer rows.Close()

	var values []DimensionValue
	for rows.Next() {
		var val DimensionValue
		err := rows.Scan(&val.ID, &val.DimensionID, &val.Value, &val.DisplayName, &val.Description, &val.Color, &val.SortOrder, &val.CreatedAt, &val.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan dimension value: %w", err)
		}
		values = append(values, val)
	}
	return values, rows.Err()
}

// GetDimensionValueByValue returns a dimension value by dimension ID and value string
func (p *PostgresDB) GetDimensionValueByValue(dimensionID int, value string) (*DimensionValue, error) {
	query := `SELECT id, dimension_id, value, display_name, description, color, sort_order, created_at, updated_at FROM dimension_values WHERE dimension_id = $1 AND value = $2`
	val := &DimensionValue{}
	err := p.db.QueryRow(query, dimensionID, value).Scan(&val.ID, &val.DimensionID, &val.Value, &val.DisplayName, &val.Description, &val.Color, &val.SortOrder, &val.CreatedAt, &val.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get dimension value: %w", err)
	}
	return val, nil
}

// GetDocumentDimensions returns all dimension values assigned to a document
func (p *PostgresDB) GetDocumentDimensions(documentID int) (map[string]DimensionValue, error) {
	query := `
		SELECT d.name, dv.id, dv.dimension_id, dv.value, dv.display_name, dv.description, dv.color, dv.sort_order, dv.created_at, dv.updated_at
		FROM document_dimensions dd
		INNER JOIN dimension_values dv ON dv.id = dd.dimension_value_id
		INNER JOIN dimensions d ON d.id = dd.dimension_id
		WHERE dd.document_id = $1
	`
	rows, err := p.db.Query(query, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query document dimensions: %w", err)
	}
	defer rows.Close()

	dimensions := make(map[string]DimensionValue)
	for rows.Next() {
		var dimName string
		var val DimensionValue
		err := rows.Scan(&dimName, &val.ID, &val.DimensionID, &val.Value, &val.DisplayName, &val.Description, &val.Color, &val.SortOrder, &val.CreatedAt, &val.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan document dimension: %w", err)
		}
		dimensions[dimName] = val
	}
	return dimensions, rows.Err()
}

// SetDocumentDimension sets a dimension value for a document
func (p *PostgresDB) SetDocumentDimension(documentID int, dimensionID int, dimensionValueID int) error {
	query := `
		INSERT INTO document_dimensions (document_id, dimension_id, dimension_value_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (document_id, dimension_id)
		DO UPDATE SET dimension_value_id = EXCLUDED.dimension_value_id, updated_at = CURRENT_TIMESTAMP
	`
	_, err := p.db.Exec(query, documentID, dimensionID, dimensionValueID)
	if err != nil {
		return fmt.Errorf("failed to set document dimension: %w", err)
	}
	return nil
}

// RemoveDocumentDimension removes a dimension value from a document
func (p *PostgresDB) RemoveDocumentDimension(documentID int, dimensionID int) error {
	query := `DELETE FROM document_dimensions WHERE document_id = $1 AND dimension_id = $2`
	_, err := p.db.Exec(query, documentID, dimensionID)
	if err != nil {
		return fmt.Errorf("failed to remove document dimension: %w", err)
	}
	return nil
}
