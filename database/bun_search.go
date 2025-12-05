package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ============================================================================
// SAVED SEARCH METHODS
// ============================================================================

// GetAllSavedSearches returns all saved searches sorted by sort_order
func (b *BunDB) GetAllSavedSearches() ([]SavedSearch, error) {
	ctx := context.Background()
	var searches []SavedSearch
	err := b.db.NewSelect().
		Model(&searches).
		Order("sort_order ASC", "name ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get saved searches: %w", err)
	}
	return searches, nil
}

// GetSavedSearchByID returns a saved search by its ID
func (b *BunDB) GetSavedSearchByID(id int) (*SavedSearch, error) {
	ctx := context.Background()
	search := &SavedSearch{}
	err := b.db.NewSelect().
		Model(search).
		Where("id = ?", id).
		Scan(ctx)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get saved search: %w", err)
	}
	return search, nil
}

// CreateSavedSearch creates a new saved search
func (b *BunDB) CreateSavedSearch(search *SavedSearch) error {
	ctx := context.Background()
	_, err := b.db.NewInsert().Model(search).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create saved search: %w", err)
	}
	return nil
}

// UpdateSavedSearch updates an existing saved search
func (b *BunDB) UpdateSavedSearch(search *SavedSearch) error {
	ctx := context.Background()
	_, err := b.db.NewUpdate().
		Model(search).
		WherePK().
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update saved search: %w", err)
	}
	return nil
}

// DeleteSavedSearch deletes a saved search by ID
func (b *BunDB) DeleteSavedSearch(id int) error {
	ctx := context.Background()
	_, err := b.db.NewDelete().
		Model((*SavedSearch)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete saved search: %w", err)
	}
	return nil
}

// ============================================================================
// SEARCH EXECUTION METHODS
// ============================================================================

// GetDocumentsByTag returns paginated documents that have a specific tag
func (b *BunDB) GetDocumentsByTag(tagID int, page, pageSize int) ([]Document, int, error) {
	ctx := context.Background()
	offset := (page - 1) * pageSize

	// Get total count
	count, err := b.db.NewSelect().
		TableExpr("documents d").
		Join("INNER JOIN document_tags dt ON dt.document_id = d.storm_id").
		Where("dt.tag_id = ?", tagID).
		Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count documents by tag: %w", err)
	}

	// Get documents
	var docs []Document
	err = b.db.NewSelect().
		TableExpr("documents d").
		ColumnExpr("d.*").
		Join("INNER JOIN document_tags dt ON dt.document_id = d.storm_id").
		Where("dt.tag_id = ?", tagID).
		Order("d.ingress_time DESC").
		Limit(pageSize).
		Offset(offset).
		Scan(ctx, &docs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get documents by tag: %w", err)
	}

	return docs, count, nil
}

// GetUntaggedDocuments returns paginated documents that have no tags
func (b *BunDB) GetUntaggedDocuments(page, pageSize int) ([]Document, int, error) {
	ctx := context.Background()
	offset := (page - 1) * pageSize

	// Get total count of untagged documents
	count, err := b.db.NewSelect().
		TableExpr("documents d").
		Where("NOT EXISTS (SELECT 1 FROM document_tags dt WHERE dt.document_id = d.storm_id)").
		Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count untagged documents: %w", err)
	}

	// Get untagged documents
	var docs []Document
	err = b.db.NewSelect().
		TableExpr("documents d").
		ColumnExpr("d.*").
		Where("NOT EXISTS (SELECT 1 FROM document_tags dt WHERE dt.document_id = d.storm_id)").
		Order("d.ingress_time DESC").
		Limit(pageSize).
		Offset(offset).
		Scan(ctx, &docs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get untagged documents: %w", err)
	}

	return docs, count, nil
}

// ExecuteSearch executes a parsed search query and returns paginated results
func (b *BunDB) ExecuteSearch(parsed *ParsedSearch, page, pageSize int) ([]Document, int, error) {
	ctx := context.Background()
	offset := (page - 1) * pageSize

	// Handle special cases
	if parsed.IsAllDocs {
		return b.GetNewestDocumentsWithPagination(page, pageSize)
	}
	if parsed.IsUntagged {
		return b.GetUntaggedDocuments(page, pageSize)
	}

	// Build dynamic query
	// Start with base query
	baseQuery := b.db.NewSelect().
		TableExpr("documents d").
		ColumnExpr("d.*")

	countQuery := b.db.NewSelect().
		TableExpr("documents d")

	// Add include tag filters (documents must have ALL these tags)
	for i, tagName := range parsed.IncludeTags {
		alias := fmt.Sprintf("it%d", i)
		tagAlias := fmt.Sprintf("t%d", i)
		joinClause := fmt.Sprintf("INNER JOIN document_tags %s ON %s.document_id = d.storm_id", alias, alias)
		tagJoin := fmt.Sprintf("INNER JOIN tags %s ON %s.id = %s.tag_id AND LOWER(%s.name) = LOWER(?)", tagAlias, tagAlias, alias, tagAlias)

		baseQuery = baseQuery.Join(joinClause).Join(tagJoin, tagName)
		countQuery = countQuery.Join(joinClause).Join(tagJoin, tagName)
	}

	// Add exclude tag filters (documents must NOT have these tags)
	for _, tagName := range parsed.ExcludeTags {
		excludeClause := `NOT EXISTS (
			SELECT 1 FROM document_tags edt
			INNER JOIN tags et ON et.id = edt.tag_id
			WHERE edt.document_id = d.storm_id AND LOWER(et.name) = LOWER(?)
		)`
		baseQuery = baseQuery.Where(excludeClause, tagName)
		countQuery = countQuery.Where(excludeClause, tagName)
	}

	// Add text search if present
	if parsed.TextTerms != "" {
		// Use full-text search if available, otherwise simple LIKE
		searchTerms := strings.Split(parsed.TextTerms, " ")
		for _, term := range searchTerms {
			term = strings.TrimSpace(term)
			if term == "" {
				continue
			}
			likeTerm := "%" + term + "%"
			textClause := "(LOWER(d.name) LIKE LOWER(?) OR LOWER(d.full_text) LIKE LOWER(?))"
			baseQuery = baseQuery.Where(textClause, likeTerm, likeTerm)
			countQuery = countQuery.Where(textClause, likeTerm, likeTerm)
		}
	}

	// Get total count
	count, err := countQuery.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	// Get results
	var docs []Document
	err = baseQuery.
		Order("d.ingress_time DESC").
		Limit(pageSize).
		Offset(offset).
		Scan(ctx, &docs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute search: %w", err)
	}

	return docs, count, nil
}
