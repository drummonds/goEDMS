package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// CreateStory creates a new story and its associated tag in a transaction.
// The tag is created with tag_group='Story' and name derived from the title.
func (p *PGDB) CreateStory(story *Story) error {
	ctx := context.Background()
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the story tag
	tagName := storyTagName(story.Title)
	storyGroup := "Story"
	now := time.Now()

	var tagID int
	err = tx.QueryRowContext(ctx, `
		INSERT INTO tags (name, color, description, tag_group, sort_order, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 0, $5, $6)
		RETURNING id`,
		tagName, "#8e44ad", story.Title, &storyGroup, now, now).Scan(&tagID)
	if err != nil {
		// Fallback for pglike
		_, execErr := tx.ExecContext(ctx, `
			INSERT INTO tags (name, color, description, tag_group, sort_order, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 0, $5, $6)`,
			tagName, "#8e44ad", story.Title, &storyGroup, now, now)
		if execErr != nil {
			return fmt.Errorf("failed to create story tag: %w", execErr)
		}
		tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE name = $1`, tagName).Scan(&tagID)
	}

	// Create the story row
	story.TagID = tagID
	story.CreatedAt = now
	story.UpdatedAt = now

	err = tx.QueryRowContext(ctx, `
		INSERT INTO stories (title, description, start_date, end_date, tag_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		story.Title, story.Description, story.StartDate, story.EndDate,
		story.TagID, story.CreatedAt, story.UpdatedAt).Scan(&story.ID)
	if err != nil {
		// Fallback for pglike
		_, execErr := tx.ExecContext(ctx, `
			INSERT INTO stories (title, description, start_date, end_date, tag_id, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			story.Title, story.Description, story.StartDate, story.EndDate,
			story.TagID, story.CreatedAt, story.UpdatedAt)
		if execErr != nil {
			return fmt.Errorf("failed to create story: %w", execErr)
		}
		tx.QueryRowContext(ctx, `SELECT id FROM stories WHERE tag_id = $1`, story.TagID).Scan(&story.ID)
	}

	return tx.Commit()
}

// GetStoryByID returns a story by its ID.
func (p *PGDB) GetStoryByID(id int) (*Story, error) {
	story, err := scanStory(p.db.QueryRowContext(context.Background(), `
		SELECT id, title, description, start_date, end_date, tag_id, created_at, updated_at
		FROM stories WHERE id = $1`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return story, err
}

// GetStoryByTagID returns a story by its tag ID.
func (p *PGDB) GetStoryByTagID(tagID int) (*Story, error) {
	story, err := scanStory(p.db.QueryRowContext(context.Background(), `
		SELECT id, title, description, start_date, end_date, tag_id, created_at, updated_at
		FROM stories WHERE tag_id = $1`, tagID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return story, err
}

// GetAllStories returns all stories ordered by start_date descending.
func (p *PGDB) GetAllStories() ([]Story, error) {
	rows, err := p.db.QueryContext(context.Background(), `
		SELECT id, title, description, start_date, end_date, tag_id, created_at, updated_at
		FROM stories
		ORDER BY CASE WHEN start_date IS NULL THEN 0 ELSE 1 END DESC,
		         start_date DESC, title ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to get all stories: %w", err)
	}
	defer rows.Close()
	return scanStoryRows(rows)
}

// GetStoriesWithMeta returns all stories with their tags, associated tags, and document counts.
func (p *PGDB) GetStoriesWithMeta() ([]StoryWithMeta, error) {
	ctx := context.Background()

	stories, err := p.GetAllStories()
	if err != nil {
		return nil, err
	}

	result := make([]StoryWithMeta, len(stories))
	for i, s := range stories {
		result[i].Story = s
		if s.StartDate != nil {
			result[i].StartDateFmt = s.StartDate.Format("2006-01-02")
		}
		if s.EndDate != nil {
			result[i].EndDateFmt = s.EndDate.Format("2006-01-02")
		}

		// Get the story's own tag
		tag, err := p.GetTagByID(s.TagID)
		if err != nil || tag == nil {
			result[i].Tag = Tag{ID: s.TagID, Name: storyTagName(s.Title)}
		} else {
			result[i].Tag = *tag
		}

		// Get associated tags
		assocTags, err := p.GetStoryTags(s.ID)
		if err != nil {
			assocTags = []Tag{}
		}
		result[i].AssociatedTags = assocTags

		// Get document count (documents with this story's tag)
		var count int
		err = p.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM document_tags WHERE tag_id = $1`, s.TagID).Scan(&count)
		if err != nil {
			count = 0
		}
		result[i].DocumentCount = count
	}

	return result, nil
}

// UpdateStory updates a story's title, description, dates, and syncs tag name/color.
func (p *PGDB) UpdateStory(story *Story) error {
	ctx := context.Background()
	now := time.Now()

	_, err := p.db.ExecContext(ctx, `
		UPDATE stories SET title = $1, description = $2, start_date = $3, end_date = $4, updated_at = $5
		WHERE id = $6`,
		story.Title, story.Description, story.StartDate, story.EndDate, now, story.ID)
	if err != nil {
		return fmt.Errorf("failed to update story: %w", err)
	}

	// Sync tag name to match title
	tagName := storyTagName(story.Title)
	_, err = p.db.ExecContext(ctx, `
		UPDATE tags SET name = $1, description = $2, updated_at = $3
		WHERE id = $4`,
		tagName, story.Title, now, story.TagID)
	if err != nil {
		return fmt.Errorf("failed to update story tag: %w", err)
	}

	return nil
}

// DeleteStory deletes a story and its associated tag (cascade cleans document_tags).
func (p *PGDB) DeleteStory(id int) error {
	ctx := context.Background()

	// Get the tag ID first
	story, err := p.GetStoryByID(id)
	if err != nil {
		return fmt.Errorf("failed to get story for deletion: %w", err)
	}
	if story == nil {
		return nil
	}

	// Delete story row
	_, err = p.db.ExecContext(ctx, `DELETE FROM stories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete story: %w", err)
	}

	// Delete the tag (cascades to document_tags)
	_, err = p.db.ExecContext(ctx, `DELETE FROM tags WHERE id = $1`, story.TagID)
	if err != nil {
		return fmt.Errorf("failed to delete story tag: %w", err)
	}

	return nil
}

// GetStoryTags returns the associated tags for a story (from story_tags junction).
func (p *PGDB) GetStoryTags(storyID int) ([]Tag, error) {
	rows, err := p.db.QueryContext(context.Background(), `
		SELECT t.id, t.name, t.color, t.description, t.tag_group, t.sort_order, t.created_at, t.updated_at
		FROM tags t
		INNER JOIN story_tags st ON st.tag_id = t.id
		WHERE st.story_id = $1
		ORDER BY t.name ASC`, storyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get story tags: %w", err)
	}
	defer rows.Close()
	return scanTagRows(rows)
}

// AddStoryTag adds an associated tag to a story.
func (p *PGDB) AddStoryTag(storyID int, tagID int) error {
	_, err := p.db.ExecContext(context.Background(), `
		INSERT INTO story_tags (story_id, tag_id)
		VALUES ($1, $2)
		ON CONFLICT (story_id, tag_id) DO NOTHING`,
		storyID, tagID)
	if err != nil {
		return fmt.Errorf("failed to add story tag: %w", err)
	}
	return nil
}

// RemoveStoryTag removes an associated tag from a story.
func (p *PGDB) RemoveStoryTag(storyID int, tagID int) error {
	_, err := p.db.ExecContext(context.Background(),
		`DELETE FROM story_tags WHERE story_id = $1 AND tag_id = $2`,
		storyID, tagID)
	if err != nil {
		return fmt.Errorf("failed to remove story tag: %w", err)
	}
	return nil
}

// AddDocumentToStory adds the story's tag (and all associated tags) to a document.
func (p *PGDB) AddDocumentToStory(documentID int, storyID int) error {
	story, err := p.GetStoryByID(storyID)
	if err != nil || story == nil {
		return fmt.Errorf("story not found: %d", storyID)
	}

	// Add the story's own tag
	if err := p.AddTagToDocument(documentID, story.TagID); err != nil {
		return fmt.Errorf("failed to add story tag to document: %w", err)
	}

	// Add all associated tags
	assocTags, err := p.GetStoryTags(storyID)
	if err != nil {
		return nil // non-fatal
	}
	for _, tag := range assocTags {
		p.AddTagToDocument(documentID, tag.ID)
	}

	return nil
}

// RemoveDocumentFromStory removes only the story tag from a document.
// Associated tags remain (they were applied upfront).
func (p *PGDB) RemoveDocumentFromStory(documentID int, storyID int) error {
	story, err := p.GetStoryByID(storyID)
	if err != nil || story == nil {
		return fmt.Errorf("story not found: %d", storyID)
	}

	return p.RemoveTagFromDocument(documentID, story.TagID)
}

// GetDocumentsWithoutStory returns documents that don't belong to any story,
// paginated and ordered by newest first.
func (p *PGDB) GetDocumentsWithoutStory(page, pageSize int) ([]Document, int, error) {
	ctx := context.Background()
	offset := (page - 1) * pageSize

	// Count
	var totalCount int
	err := p.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM documents d
		WHERE NOT EXISTS (
			SELECT 1 FROM document_tags dt
			INNER JOIN tags t ON t.id = dt.tag_id
			WHERE dt.document_id = d.id AND t.tag_group = 'Story'
		)`).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count unstoried documents: %w", err)
	}

	// Fetch
	rows, err := p.db.QueryContext(ctx, `
		SELECT `+docColumnsAliased+`
		FROM documents d
		WHERE NOT EXISTS (
			SELECT 1 FROM document_tags dt
			INNER JOIN tags t ON t.id = dt.tag_id
			WHERE dt.document_id = d.id AND t.tag_group = 'Story'
		)
		ORDER BY d.ingress_time DESC
		LIMIT $1 OFFSET $2`, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get unstoried documents: %w", err)
	}
	defer rows.Close()

	docs, err := scanDocumentRows(rows)
	return docs, totalCount, err
}

// scanStory scans a single row into a Story.
func scanStory(row interface{ Scan(dest ...any) error }) (*Story, error) {
	s := &Story{}
	var startDate, endDate nullTimeScanner
	var createdAt, updatedAt timeScanner
	err := row.Scan(&s.ID, &s.Title, &s.Description, &startDate, &endDate,
		&s.TagID, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	s.StartDate = startDate.Time
	s.EndDate = endDate.Time
	s.CreatedAt = createdAt.Time
	s.UpdatedAt = updatedAt.Time
	return s, nil
}

// scanStoryRows scans multiple rows into Stories.
func scanStoryRows(rows *sql.Rows) ([]Story, error) {
	var stories []Story
	for rows.Next() {
		s, err := scanStory(rows)
		if err != nil {
			return nil, err
		}
		stories = append(stories, *s)
	}
	return stories, rows.Err()
}

// storyTagName converts a story title to a tag name (spaces to hyphens).
func storyTagName(title string) string {
	return strings.ReplaceAll(strings.TrimSpace(title), " ", "-")
}
