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
func (p *PGDB) GetAllSavedSearches() ([]SavedSearch, error) {
	rows, err := p.db.QueryContext(context.Background(), `
		SELECT id, name, description, query, icon, sort_order, is_system, created_at, updated_at
		FROM saved_searches ORDER BY sort_order ASC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to get saved searches: %w", err)
	}
	defer rows.Close()
	return scanSavedSearchRows(rows)
}

// GetSavedSearchByID returns a saved search by its ID
func (p *PGDB) GetSavedSearchByID(id int) (*SavedSearch, error) {
	s, err := scanSavedSearch(p.db.QueryRowContext(context.Background(), `
		SELECT id, name, description, query, icon, sort_order, is_system, created_at, updated_at
		FROM saved_searches WHERE id = $1`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// CreateSavedSearch creates a new saved search
func (p *PGDB) CreateSavedSearch(search *SavedSearch) error {
	ctx := context.Background()
	err := p.db.QueryRowContext(ctx, `
		INSERT INTO saved_searches (name, description, query, icon, sort_order, is_system, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		search.Name, search.Description, search.Query, search.Icon,
		search.SortOrder, search.IsSystem, search.CreatedAt, search.UpdatedAt).Scan(&search.ID)
	if err != nil {
		// Fallback for pglike
		_, execErr := p.db.ExecContext(ctx, `
			INSERT INTO saved_searches (name, description, query, icon, sort_order, is_system, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			search.Name, search.Description, search.Query, search.Icon,
			search.SortOrder, search.IsSystem, search.CreatedAt, search.UpdatedAt)
		if execErr != nil {
			return fmt.Errorf("failed to create saved search: %w", execErr)
		}
		p.db.QueryRowContext(ctx, `SELECT id FROM saved_searches WHERE name = $1`, search.Name).Scan(&search.ID)
	}
	return nil
}

// UpdateSavedSearch updates an existing saved search
func (p *PGDB) UpdateSavedSearch(search *SavedSearch) error {
	_, err := p.db.ExecContext(context.Background(), `
		UPDATE saved_searches SET
			name = $1, description = $2, query = $3, icon = $4,
			sort_order = $5, is_system = $6, updated_at = $7
		WHERE id = $8`,
		search.Name, search.Description, search.Query, search.Icon,
		search.SortOrder, search.IsSystem, search.UpdatedAt, search.ID)
	if err != nil {
		return fmt.Errorf("failed to update saved search: %w", err)
	}
	return nil
}

// DeleteSavedSearch deletes a saved search by ID
func (p *PGDB) DeleteSavedSearch(id int) error {
	_, err := p.db.ExecContext(context.Background(),
		`DELETE FROM saved_searches WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete saved search: %w", err)
	}
	return nil
}

// scanSavedSearch scans a single row into a SavedSearch.
// Uses timeScanner to handle both time.Time (PostgreSQL) and string (pglike/SQLite) timestamps.
func scanSavedSearch(row interface{ Scan(dest ...any) error }) (*SavedSearch, error) {
	s := &SavedSearch{}
	var createdAt, updatedAt timeScanner
	err := row.Scan(&s.ID, &s.Name, &s.Description, &s.Query, &s.Icon,
		&s.SortOrder, &s.IsSystem, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	s.CreatedAt = createdAt.Time
	s.UpdatedAt = updatedAt.Time
	return s, nil
}

// scanSavedSearchRows scans multiple rows into SavedSearches
func scanSavedSearchRows(rows *sql.Rows) ([]SavedSearch, error) {
	var searches []SavedSearch
	for rows.Next() {
		s, err := scanSavedSearch(rows)
		if err != nil {
			return nil, err
		}
		searches = append(searches, *s)
	}
	return searches, rows.Err()
}

// ============================================================================
// SEARCH EXECUTION METHODS
// ============================================================================

// GetDocumentsByTag returns paginated documents that have a specific tag
func (p *PGDB) GetDocumentsByTag(tagID int, page, pageSize int) ([]Document, int, error) {
	ctx := context.Background()
	offset := (page - 1) * pageSize

	var count int
	err := p.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM documents d
		INNER JOIN document_tags dt ON dt.document_id = d.id
		WHERE dt.tag_id = $1`, tagID).Scan(&count)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count documents by tag: %w", err)
	}

	rows, err := p.db.QueryContext(ctx, `
		SELECT d.id, d.name, d.path, d.ingress_time, d.folder, d.hash, d.ulid,
		       d.document_type, d.full_text, d.url
		FROM documents d
		INNER JOIN document_tags dt ON dt.document_id = d.id
		WHERE dt.tag_id = $1
		ORDER BY d.ingress_time DESC
		LIMIT $2 OFFSET $3`, tagID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get documents by tag: %w", err)
	}
	defer rows.Close()

	docs, err := scanDocumentRows(rows)
	return docs, count, err
}

// GetUntaggedDocuments returns paginated documents that have no tags
func (p *PGDB) GetUntaggedDocuments(page, pageSize int) ([]Document, int, error) {
	ctx := context.Background()
	offset := (page - 1) * pageSize

	var count int
	err := p.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM documents d
		WHERE NOT EXISTS (SELECT 1 FROM document_tags dt WHERE dt.document_id = d.id)`).Scan(&count)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count untagged documents: %w", err)
	}

	rows, err := p.db.QueryContext(ctx, `
		SELECT d.id, d.name, d.path, d.ingress_time, d.folder, d.hash, d.ulid,
		       d.document_type, d.full_text, d.url
		FROM documents d
		WHERE NOT EXISTS (SELECT 1 FROM document_tags dt WHERE dt.document_id = d.id)
		ORDER BY d.ingress_time DESC
		LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get untagged documents: %w", err)
	}
	defer rows.Close()

	docs, err := scanDocumentRows(rows)
	return docs, count, err
}

// ExecuteSearch executes a parsed search query and returns paginated results
func (p *PGDB) ExecuteSearch(parsed *ParsedSearch, page, pageSize int) ([]Document, int, error) {
	ctx := context.Background()
	offset := (page - 1) * pageSize

	if parsed.IsAllDocs {
		return p.GetNewestDocumentsWithPagination(page, pageSize)
	}
	if parsed.IsUntagged {
		return p.GetUntaggedDocuments(page, pageSize)
	}

	// Build dynamic query with parameterized placeholders
	var (
		joins      []string
		conditions []string
		args       []interface{}
		argIdx     = 1 // PostgreSQL $1, $2, ... placeholders
	)

	// Add include tag filters (documents must have ALL these tags)
	for i, tagName := range parsed.IncludeTags {
		dtAlias := fmt.Sprintf("it%d", i)
		tAlias := fmt.Sprintf("t%d", i)
		joins = append(joins,
			fmt.Sprintf("INNER JOIN document_tags %s ON %s.document_id = d.id", dtAlias, dtAlias))
		joins = append(joins,
			fmt.Sprintf("INNER JOIN tags %s ON %s.id = %s.tag_id AND LOWER(%s.name) = LOWER($%d)",
				tAlias, tAlias, dtAlias, tAlias, argIdx))
		args = append(args, tagName)
		argIdx++
	}

	// Add exclude tag filters
	for _, tagName := range parsed.ExcludeTags {
		conditions = append(conditions, fmt.Sprintf(`
			NOT EXISTS (
				SELECT 1 FROM document_tags edt
				INNER JOIN tags et ON et.id = edt.tag_id
				WHERE edt.document_id = d.id AND LOWER(et.name) = LOWER($%d)
			)`, argIdx))
		args = append(args, tagName)
		argIdx++
	}

	// Add text search
	if parsed.TextTerms != "" {
		searchTerms := strings.Split(parsed.TextTerms, " ")
		for _, term := range searchTerms {
			term = strings.TrimSpace(term)
			if term == "" {
				continue
			}
			likeTerm := "%" + term + "%"
			conditions = append(conditions,
				fmt.Sprintf("(LOWER(d.name) LIKE LOWER($%d) OR LOWER(d.full_text) LIKE LOWER($%d))",
					argIdx, argIdx+1))
			args = append(args, likeTerm, likeTerm)
			argIdx += 2
		}
	}

	// Build WHERE clause
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}
	joinClause := strings.Join(joins, " ")

	// Get total count
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM documents d %s %s", joinClause, whereClause)
	var count int
	if err := p.db.QueryRowContext(ctx, countSQL, args...).Scan(&count); err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	// Get results
	selectSQL := fmt.Sprintf(`
		SELECT d.id, d.name, d.path, d.ingress_time, d.folder, d.hash, d.ulid,
		       d.document_type, d.full_text, d.url
		FROM documents d %s %s
		ORDER BY d.ingress_time DESC
		LIMIT $%d OFFSET $%d`, joinClause, whereClause, argIdx, argIdx+1)
	args = append(args, pageSize, offset)

	rows, err := p.db.QueryContext(ctx, selectSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute search: %w", err)
	}
	defer rows.Close()

	docs, err := scanDocumentRows(rows)
	return docs, count, err
}
