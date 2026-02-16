package database

import "time"

// Story represents a grouping of documents (e.g. a house purchase, medical episode).
// Each story creates a tag with tag_group='Story', so a document belongs to at most one story.
type Story struct {
	ID          int        `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	StartDate   *time.Time `json:"start_date,omitempty"`
	EndDate     *time.Time `json:"end_date,omitempty"`
	TagID       int        `json:"tag_id"` // references the story's own tag
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// StoryWithMeta includes the story's tag, associated tags, and document count for display.
type StoryWithMeta struct {
	Story
	Tag            Tag   // The story's own tag (has color, name etc.)
	AssociatedTags []Tag // Tags from story_tags junction
	DocumentCount  int
	StartDateFmt   string // Pre-formatted "2006-01-02" for templates (avoids *time.Time filter issues)
	EndDateFmt     string
}
