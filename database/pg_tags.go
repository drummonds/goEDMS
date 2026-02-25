package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ============================================================================
// TAG METHODS
// ============================================================================

// CreateTag creates a new tag
func (p *PGDB) CreateTag(tag *Tag) error {
	ctx := context.Background()
	err := p.db.QueryRowContext(ctx, `
		INSERT INTO tags (name, color, description, tag_group, sort_order, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		tag.Name, tag.Color, tag.Description, tag.TagGroup, tag.SortOrder,
		time.Now(), time.Now()).Scan(&tag.ID)
	if err != nil {
		// Fallback for pglike which may not support RETURNING
		_, execErr := p.db.ExecContext(ctx, `
			INSERT INTO tags (name, color, description, tag_group, sort_order, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			tag.Name, tag.Color, tag.Description, tag.TagGroup, tag.SortOrder,
			time.Now(), time.Now())
		if execErr != nil {
			return fmt.Errorf("failed to create tag: %w", execErr)
		}
		// Fetch the ID
		p.db.QueryRowContext(ctx, `SELECT id FROM tags WHERE name = $1`, tag.Name).Scan(&tag.ID)
	}
	return nil
}

// GetAllTags returns all tags sorted by group and sort_order
// Uses CASE WHEN workaround for NULLS FIRST (not yet supported by go-postgres)
func (p *PGDB) GetAllTags() ([]Tag, error) {
	ctx := context.Background()
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, name, color, description, tag_group, sort_order, created_at, updated_at
		FROM tags
		ORDER BY CASE WHEN tag_group IS NULL THEN 0 ELSE 1 END,
		         tag_group ASC, sort_order ASC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all tags: %w", err)
	}
	defer rows.Close()
	return scanTagRows(rows)
}

// GetTagGroups returns all distinct tag group names
func (p *PGDB) GetTagGroups() ([]string, error) {
	ctx := context.Background()
	rows, err := p.db.QueryContext(ctx, `
		SELECT DISTINCT tag_group FROM tags
		WHERE tag_group IS NOT NULL
		ORDER BY tag_group ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to get tag groups: %w", err)
	}
	defer rows.Close()

	var groups []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// GetTagByID returns a tag by its ID
func (p *PGDB) GetTagByID(id int) (*Tag, error) {
	tag, err := scanTag(p.db.QueryRowContext(context.Background(), `
		SELECT id, name, color, description, tag_group, sort_order, created_at, updated_at
		FROM tags WHERE id = $1`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return tag, err
}

// GetTagByName returns a tag by its name, falling back to alias lookup
func (p *PGDB) GetTagByName(name string) (*Tag, error) {
	tag, err := scanTag(p.db.QueryRowContext(context.Background(), `
		SELECT id, name, color, description, tag_group, sort_order, created_at, updated_at
		FROM tags WHERE name = $1`, name))
	if err == sql.ErrNoRows {
		// Fallback: check tag_aliases
		tag, err = scanTag(p.db.QueryRowContext(context.Background(), `
			SELECT t.id, t.name, t.color, t.description, t.tag_group, t.sort_order, t.created_at, t.updated_at
			FROM tags t JOIN tag_aliases ta ON ta.tag_id = t.id
			WHERE ta.alias_name = $1`, name))
		if err == sql.ErrNoRows {
			return nil, nil
		}
	}
	return tag, err
}

// UpdateTag updates an existing tag. If the name changed, the old name is
// saved as an alias so that .tags.json sidecars with the old name still resolve.
func (p *PGDB) UpdateTag(tag *Tag) error {
	ctx := context.Background()

	// Fetch current name to detect renames
	var oldName string
	err := p.db.QueryRowContext(ctx,
		`SELECT name FROM tags WHERE id = $1`, tag.ID).Scan(&oldName)
	if err != nil {
		return fmt.Errorf("failed to fetch current tag name: %w", err)
	}

	_, err = p.db.ExecContext(ctx, `
		UPDATE tags SET name = $1, color = $2, description = $3,
			tag_group = $4, sort_order = $5, updated_at = $6
		WHERE id = $7`,
		tag.Name, tag.Color, tag.Description, tag.TagGroup, tag.SortOrder,
		time.Now(), tag.ID)
	if err != nil {
		return fmt.Errorf("failed to update tag: %w", err)
	}

	// If renamed, record old name as alias
	if oldName != tag.Name {
		_, err = p.db.ExecContext(ctx, `
			INSERT INTO tag_aliases (tag_id, alias_name) VALUES ($1, $2)
			ON CONFLICT (alias_name) DO NOTHING`, tag.ID, oldName)
		if err != nil {
			return fmt.Errorf("failed to insert tag alias: %w", err)
		}
	}

	return nil
}

// DeleteTag deletes a tag (cascade removes document associations)
func (p *PGDB) DeleteTag(id int) error {
	_, err := p.db.ExecContext(context.Background(),
		`DELETE FROM tags WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete tag: %w", err)
	}
	return nil
}

// GetTagsForDocument returns all tags associated with a document
func (p *PGDB) GetTagsForDocument(documentID int) ([]Tag, error) {
	rows, err := p.db.QueryContext(context.Background(), `
		SELECT t.id, t.name, t.color, t.description, t.tag_group, t.sort_order, t.created_at, t.updated_at
		FROM tags t
		INNER JOIN document_tags dt ON dt.tag_id = t.id
		WHERE dt.document_id = $1
		ORDER BY CASE WHEN t.tag_group IS NULL THEN 0 ELSE 1 END,
		         t.tag_group ASC, t.sort_order ASC, t.name ASC`, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags for document: %w", err)
	}
	defer rows.Close()
	return scanTagRows(rows)
}

// AddTagToDocument associates a tag with a document
func (p *PGDB) AddTagToDocument(documentID int, tagID int) error {
	_, err := p.db.ExecContext(context.Background(), `
		INSERT INTO document_tags (document_id, tag_id, created_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (document_id, tag_id) DO NOTHING`,
		documentID, tagID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to add tag to document: %w", err)
	}
	return nil
}

// RemoveTagFromDocument removes a tag association from a document
func (p *PGDB) RemoveTagFromDocument(documentID int, tagID int) error {
	_, err := p.db.ExecContext(context.Background(),
		`DELETE FROM document_tags WHERE document_id = $1 AND tag_id = $2`,
		documentID, tagID)
	if err != nil {
		return fmt.Errorf("failed to remove tag from document: %w", err)
	}
	return nil
}

// GetTagUsageCount returns the number of documents using a specific tag
func (p *PGDB) GetTagUsageCount(tagID int) (int, error) {
	var count int
	err := p.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM document_tags WHERE tag_id = $1`, tagID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get tag usage count: %w", err)
	}
	return count, nil
}

// GetTopTagsByUsage returns the most-used tags sorted by document count descending.
func (p *PGDB) GetTopTagsByUsage(limit int) ([]TagWithCount, error) {
	ctx := context.Background()
	rows, err := p.db.QueryContext(ctx, `
		SELECT t.id, t.name, t.color, t.description, t.tag_group, t.sort_order, t.created_at, t.updated_at,
		       COUNT(dt.document_id) AS document_count
		FROM tags t
		JOIN document_tags dt ON dt.tag_id = t.id
		GROUP BY t.id, t.name, t.color, t.description, t.tag_group, t.sort_order, t.created_at, t.updated_at
		ORDER BY document_count DESC, t.name ASC
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get top tags by usage: %w", err)
	}
	defer rows.Close()

	var result []TagWithCount
	for rows.Next() {
		var tc TagWithCount
		var createdAt, updatedAt timeScanner
		if err := rows.Scan(&tc.ID, &tc.Name, &tc.Color, &tc.Description,
			&tc.TagGroup, &tc.SortOrder, &createdAt, &updatedAt,
			&tc.DocumentCount); err != nil {
			return nil, err
		}
		tc.CreatedAt = createdAt.Time
		tc.UpdatedAt = updatedAt.Time
		result = append(result, tc)
	}
	return result, rows.Err()
}

// scanTag scans a single row into a Tag.
// Uses timeScanner to handle both time.Time (PostgreSQL) and string (pglike/SQLite) timestamps.
func scanTag(row interface{ Scan(dest ...any) error }) (*Tag, error) {
	tag := &Tag{}
	var createdAt, updatedAt timeScanner
	err := row.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.Description,
		&tag.TagGroup, &tag.SortOrder, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	tag.CreatedAt = createdAt.Time
	tag.UpdatedAt = updatedAt.Time
	return tag, nil
}

// scanTagRows scans multiple rows into Tags
func scanTagRows(rows *sql.Rows) ([]Tag, error) {
	var tags []Tag
	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		tags = append(tags, *tag)
	}
	return tags, rows.Err()
}

// GetAllTagAliases returns all tag aliases with resolved tag names
func (p *PGDB) GetAllTagAliases() ([]TagAliasEntry, error) {
	rows, err := p.db.QueryContext(context.Background(), `
		SELECT ta.alias_name, t.name
		FROM tag_aliases ta JOIN tags t ON ta.tag_id = t.id
		ORDER BY t.name, ta.alias_name`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all tag aliases: %w", err)
	}
	defer rows.Close()

	var aliases []TagAliasEntry
	for rows.Next() {
		var e TagAliasEntry
		if err := rows.Scan(&e.AliasName, &e.TagName); err != nil {
			return nil, err
		}
		aliases = append(aliases, e)
	}
	return aliases, rows.Err()
}

// InsertTagAlias inserts a tag alias (does nothing on conflict)
func (p *PGDB) InsertTagAlias(tagID int, aliasName string) error {
	_, err := p.db.ExecContext(context.Background(), `
		INSERT INTO tag_aliases (tag_id, alias_name) VALUES ($1, $2)
		ON CONFLICT (alias_name) DO NOTHING`, tagID, aliasName)
	if err != nil {
		return fmt.Errorf("failed to insert tag alias: %w", err)
	}
	return nil
}

// ============================================================================
// DIMENSION METHODS
// ============================================================================

// GetAllDimensions returns all dimension definitions
func (p *PGDB) GetAllDimensions() ([]Dimension, error) {
	ctx := context.Background()
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, name, display_name, description, dimension_type, is_required, created_at, updated_at
		FROM dimensions ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all dimensions: %w", err)
	}
	defer rows.Close()

	var dimensions []Dimension
	for rows.Next() {
		var d Dimension
		var createdAt, updatedAt timeScanner
		if err := rows.Scan(&d.ID, &d.Name, &d.DisplayName, &d.Description,
			&d.DimensionType, &d.IsRequired, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		d.CreatedAt = createdAt.Time
		d.UpdatedAt = updatedAt.Time
		dimensions = append(dimensions, d)
	}
	return dimensions, rows.Err()
}

// GetDimensionByID returns a dimension by its ID
func (p *PGDB) GetDimensionByID(id int) (*Dimension, error) {
	d := &Dimension{}
	var createdAt, updatedAt timeScanner
	err := p.db.QueryRowContext(context.Background(), `
		SELECT id, name, display_name, description, dimension_type, is_required, created_at, updated_at
		FROM dimensions WHERE id = $1`, id).Scan(
		&d.ID, &d.Name, &d.DisplayName, &d.Description,
		&d.DimensionType, &d.IsRequired, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get dimension by ID: %w", err)
	}
	d.CreatedAt = createdAt.Time
	d.UpdatedAt = updatedAt.Time
	return d, nil
}

// GetDimensionByName returns a dimension by its name
func (p *PGDB) GetDimensionByName(name string) (*Dimension, error) {
	d := &Dimension{}
	var createdAt, updatedAt timeScanner
	err := p.db.QueryRowContext(context.Background(), `
		SELECT id, name, display_name, description, dimension_type, is_required, created_at, updated_at
		FROM dimensions WHERE name = $1`, name).Scan(
		&d.ID, &d.Name, &d.DisplayName, &d.Description,
		&d.DimensionType, &d.IsRequired, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get dimension by name: %w", err)
	}
	d.CreatedAt = createdAt.Time
	d.UpdatedAt = updatedAt.Time
	return d, nil
}

// GetDimensionValues returns all possible values for a dimension
func (p *PGDB) GetDimensionValues(dimensionID int) ([]DimensionValue, error) {
	rows, err := p.db.QueryContext(context.Background(), `
		SELECT id, dimension_id, value, display_name, description, color, sort_order, created_at, updated_at
		FROM dimension_values WHERE dimension_id = $1 ORDER BY sort_order ASC`, dimensionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get dimension values: %w", err)
	}
	defer rows.Close()

	var values []DimensionValue
	for rows.Next() {
		var v DimensionValue
		var createdAt, updatedAt timeScanner
		if err := rows.Scan(&v.ID, &v.DimensionID, &v.Value, &v.DisplayName,
			&v.Description, &v.Color, &v.SortOrder, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		v.CreatedAt = createdAt.Time
		v.UpdatedAt = updatedAt.Time
		values = append(values, v)
	}
	return values, rows.Err()
}

// GetDimensionValueByValue returns a dimension value by dimension ID and value string
func (p *PGDB) GetDimensionValueByValue(dimensionID int, value string) (*DimensionValue, error) {
	v := &DimensionValue{}
	var createdAt, updatedAt timeScanner
	err := p.db.QueryRowContext(context.Background(), `
		SELECT id, dimension_id, value, display_name, description, color, sort_order, created_at, updated_at
		FROM dimension_values WHERE dimension_id = $1 AND value = $2`, dimensionID, value).Scan(
		&v.ID, &v.DimensionID, &v.Value, &v.DisplayName,
		&v.Description, &v.Color, &v.SortOrder, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get dimension value: %w", err)
	}
	v.CreatedAt = createdAt.Time
	v.UpdatedAt = updatedAt.Time
	return v, nil
}

// GetDocumentDimensions returns all dimension values assigned to a document
func (p *PGDB) GetDocumentDimensions(documentID int) (map[string]DimensionValue, error) {
	rows, err := p.db.QueryContext(context.Background(), `
		SELECT dv.id, dv.dimension_id, dv.value, dv.display_name, dv.description,
		       dv.color, dv.sort_order, dv.created_at, dv.updated_at,
		       d.name as dimension_name
		FROM document_dimensions dd
		INNER JOIN dimension_values dv ON dv.id = dd.dimension_value_id
		INNER JOIN dimensions d ON d.id = dd.dimension_id
		WHERE dd.document_id = $1`, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get document dimensions: %w", err)
	}
	defer rows.Close()

	dimensions := make(map[string]DimensionValue)
	for rows.Next() {
		var v DimensionValue
		var dimName string
		var createdAt, updatedAt timeScanner
		if err := rows.Scan(&v.ID, &v.DimensionID, &v.Value, &v.DisplayName,
			&v.Description, &v.Color, &v.SortOrder, &createdAt, &updatedAt,
			&dimName); err != nil {
			return nil, err
		}
		v.CreatedAt = createdAt.Time
		v.UpdatedAt = updatedAt.Time
		dimensions[dimName] = v
	}
	return dimensions, rows.Err()
}

// SetDocumentDimension sets a dimension value for a document (replaces existing)
func (p *PGDB) SetDocumentDimension(documentID int, dimensionID int, dimensionValueID int) error {
	_, err := p.db.ExecContext(context.Background(), `
		INSERT INTO document_dimensions (document_id, dimension_id, dimension_value_id, created_at, updated_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (document_id, dimension_id) DO UPDATE SET
			dimension_value_id = EXCLUDED.dimension_value_id,
			updated_at = CURRENT_TIMESTAMP`,
		documentID, dimensionID, dimensionValueID)
	if err != nil {
		return fmt.Errorf("failed to set document dimension: %w", err)
	}
	return nil
}

// RemoveDocumentDimension removes a dimension value from a document
func (p *PGDB) RemoveDocumentDimension(documentID int, dimensionID int) error {
	_, err := p.db.ExecContext(context.Background(),
		`DELETE FROM document_dimensions WHERE document_id = $1 AND dimension_id = $2`,
		documentID, dimensionID)
	if err != nil {
		return fmt.Errorf("failed to remove document dimension: %w", err)
	}
	return nil
}
