package database

import (
	"database/sql"
	"fmt"
	"strings"
)

// ============================================================================
// SAVED SEARCH METHODS - PostgreSQL Implementation
// ============================================================================

// GetAllSavedSearches returns all saved searches sorted by sort_order
func (p *PostgresDB) GetAllSavedSearches() ([]SavedSearch, error) {
	query := `SELECT id, name, description, query, icon, sort_order, is_system, created_at, updated_at
	          FROM saved_searches ORDER BY sort_order ASC, name ASC`
	rows, err := p.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get saved searches: %w", err)
	}
	defer rows.Close()

	var searches []SavedSearch
	for rows.Next() {
		var s SavedSearch
		err := rows.Scan(&s.ID, &s.Name, &s.Description, &s.Query, &s.Icon, &s.SortOrder, &s.IsSystem, &s.CreatedAt, &s.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan saved search: %w", err)
		}
		searches = append(searches, s)
	}
	return searches, rows.Err()
}

// GetSavedSearchByID returns a saved search by its ID
func (p *PostgresDB) GetSavedSearchByID(id int) (*SavedSearch, error) {
	query := `SELECT id, name, description, query, icon, sort_order, is_system, created_at, updated_at
	          FROM saved_searches WHERE id = $1`
	s := &SavedSearch{}
	err := p.db.QueryRow(query, id).Scan(&s.ID, &s.Name, &s.Description, &s.Query, &s.Icon, &s.SortOrder, &s.IsSystem, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get saved search: %w", err)
	}
	return s, nil
}

// CreateSavedSearch creates a new saved search
func (p *PostgresDB) CreateSavedSearch(search *SavedSearch) error {
	query := `INSERT INTO saved_searches (name, description, query, icon, sort_order, is_system)
	          VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at, updated_at`
	err := p.db.QueryRow(query, search.Name, search.Description, search.Query, search.Icon, search.SortOrder, search.IsSystem).
		Scan(&search.ID, &search.CreatedAt, &search.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create saved search: %w", err)
	}
	return nil
}

// UpdateSavedSearch updates an existing saved search
func (p *PostgresDB) UpdateSavedSearch(search *SavedSearch) error {
	query := `UPDATE saved_searches SET name = $1, description = $2, query = $3, icon = $4, sort_order = $5, updated_at = CURRENT_TIMESTAMP
	          WHERE id = $6`
	_, err := p.db.Exec(query, search.Name, search.Description, search.Query, search.Icon, search.SortOrder, search.ID)
	if err != nil {
		return fmt.Errorf("failed to update saved search: %w", err)
	}
	return nil
}

// DeleteSavedSearch deletes a saved search by ID
func (p *PostgresDB) DeleteSavedSearch(id int) error {
	query := `DELETE FROM saved_searches WHERE id = $1`
	_, err := p.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete saved search: %w", err)
	}
	return nil
}

// ============================================================================
// SEARCH EXECUTION METHODS - PostgreSQL Implementation
// ============================================================================

// GetDocumentsByTag returns paginated documents that have a specific tag
func (p *PostgresDB) GetDocumentsByTag(tagID int, page, pageSize int) ([]Document, int, error) {
	offset := (page - 1) * pageSize

	// Get total count
	var count int
	countQuery := `SELECT COUNT(*) FROM documents d
	               INNER JOIN document_tags dt ON dt.document_id = d.id
	               WHERE dt.tag_id = $1`
	err := p.db.QueryRow(countQuery, tagID).Scan(&count)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count documents by tag: %w", err)
	}

	// Get documents
	query := `SELECT d.id, d.name, d.path, d.ingress_time, d.folder, d.hash, d.ulid, d.document_type, d.full_text, d.url
	          FROM documents d
	          INNER JOIN document_tags dt ON dt.document_id = d.id
	          WHERE dt.tag_id = $1
	          ORDER BY d.ingress_time DESC
	          LIMIT $2 OFFSET $3`
	rows, err := p.db.Query(query, tagID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get documents by tag: %w", err)
	}
	defer rows.Close()

	docs, err := scanDocuments(rows)
	if err != nil {
		return nil, 0, err
	}

	return docs, count, nil
}

// GetUntaggedDocuments returns paginated documents that have no tags
func (p *PostgresDB) GetUntaggedDocuments(page, pageSize int) ([]Document, int, error) {
	offset := (page - 1) * pageSize

	// Get total count
	var count int
	countQuery := `SELECT COUNT(*) FROM documents d
	               WHERE NOT EXISTS (SELECT 1 FROM document_tags dt WHERE dt.document_id = d.id)`
	err := p.db.QueryRow(countQuery).Scan(&count)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count untagged documents: %w", err)
	}

	// Get documents
	query := `SELECT d.id, d.name, d.path, d.ingress_time, d.folder, d.hash, d.ulid, d.document_type, d.full_text, d.url
	          FROM documents d
	          WHERE NOT EXISTS (SELECT 1 FROM document_tags dt WHERE dt.document_id = d.id)
	          ORDER BY d.ingress_time DESC
	          LIMIT $1 OFFSET $2`
	rows, err := p.db.Query(query, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get untagged documents: %w", err)
	}
	defer rows.Close()

	docs, err := scanDocuments(rows)
	if err != nil {
		return nil, 0, err
	}

	return docs, count, nil
}

// ExecuteSearch executes a parsed search query and returns paginated results
func (p *PostgresDB) ExecuteSearch(parsed *ParsedSearch, page, pageSize int) ([]Document, int, error) {
	offset := (page - 1) * pageSize

	// Handle special cases
	if parsed.IsAllDocs {
		return p.GetNewestDocumentsWithPagination(page, pageSize)
	}
	if parsed.IsUntagged {
		return p.GetUntaggedDocuments(page, pageSize)
	}

	// Build dynamic query
	var whereClauses []string
	var joinClauses []string
	var args []interface{}
	argNum := 1

	// Add include tag filters
	for i, tagName := range parsed.IncludeTags {
		alias := fmt.Sprintf("it%d", i)
		tagAlias := fmt.Sprintf("t%d", i)
		joinClauses = append(joinClauses, fmt.Sprintf("INNER JOIN document_tags %s ON %s.document_id = d.id", alias, alias))
		joinClauses = append(joinClauses, fmt.Sprintf("INNER JOIN tags %s ON %s.id = %s.tag_id AND LOWER(%s.name) = LOWER($%d)", tagAlias, tagAlias, alias, tagAlias, argNum))
		args = append(args, tagName)
		argNum++
	}

	// Add exclude tag filters
	for _, tagName := range parsed.ExcludeTags {
		excludeClause := fmt.Sprintf(`NOT EXISTS (
			SELECT 1 FROM document_tags edt
			INNER JOIN tags et ON et.id = edt.tag_id
			WHERE edt.document_id = d.id AND LOWER(et.name) = LOWER($%d)
		)`, argNum)
		whereClauses = append(whereClauses, excludeClause)
		args = append(args, tagName)
		argNum++
	}

	// Add text search
	if parsed.TextTerms != "" {
		searchTerms := strings.Split(parsed.TextTerms, " ")
		for _, term := range searchTerms {
			term = strings.TrimSpace(term)
			if term == "" {
				continue
			}
			textClause := fmt.Sprintf("(LOWER(d.name) LIKE LOWER($%d) OR LOWER(d.full_text) LIKE LOWER($%d))", argNum, argNum+1)
			whereClauses = append(whereClauses, textClause)
			likeTerm := "%" + term + "%"
			args = append(args, likeTerm, likeTerm)
			argNum += 2
		}
	}

	// Build WHERE clause
	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Build JOIN clause
	joinSQL := strings.Join(joinClauses, " ")

	// Count query
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM documents d %s %s`, joinSQL, whereSQL)
	var count int
	err := p.db.QueryRow(countQuery, args...).Scan(&count)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	// Results query
	resultsQuery := fmt.Sprintf(`SELECT d.id, d.name, d.path, d.ingress_time, d.folder, d.hash, d.ulid, d.document_type, d.full_text, d.url
	                              FROM documents d %s %s
	                              ORDER BY d.ingress_time DESC
	                              LIMIT $%d OFFSET $%d`, joinSQL, whereSQL, argNum, argNum+1)
	args = append(args, pageSize, offset)

	rows, err := p.db.Query(resultsQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to execute search: %w", err)
	}
	defer rows.Close()

	docs, err := scanDocuments(rows)
	if err != nil {
		return nil, 0, err
	}

	return docs, count, nil
}

// Note: scanDocuments helper is defined in postgres_database.go
