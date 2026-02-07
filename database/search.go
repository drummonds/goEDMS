package database

import (
	"regexp"
	"strings"
	"time"
)

// SavedSearch represents a saved search query
type SavedSearch struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Query       string    `json:"query"`           // Unified search query (text #tag ~exclude)
	Icon        string    `json:"icon"`             // Emoji icon
	SortOrder   int       `json:"sort_order"`       // Display order
	IsSystem    bool      `json:"is_system"`        // True for built-in searches
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ParsedSearch represents a parsed search query
type ParsedSearch struct {
	TextTerms   string   // Free text to search for
	IncludeTags []string // Tags to include (prefixed with #)
	ExcludeTags []string // Tags to exclude (prefixed with ~)
	IsUntagged  bool     // Special: show only untagged documents
	IsAllDocs   bool     // Special: show all documents
}

// SearchResult extends Document with search-related info
type SearchResult struct {
	Document
	MatchScore float64 `json:"match_score,omitempty"`
}

// tagPattern matches #tagname or ~tagname (allowing alphanumeric, dash, underscore)
var tagPattern = regexp.MustCompile(`([#~])([a-zA-Z0-9_\-]+)`)

// ParseSearchQuery parses a unified search query string
// Syntax:
//   - text: full-text search terms
//   - #tagname: include documents with this tag
//   - ~tagname: exclude documents with this tag
//   - Special: "*" means all documents
//   - Special: "!untagged" means only untagged documents
//
// Examples:
//   - "cancer" -> text search for "cancer"
//   - "#Invoice" -> documents with Invoice tag
//   - "~2025" -> documents without 2025 tag
//   - "cancer #Invoice ~Draft" -> text "cancer", has Invoice, no Draft
func ParseSearchQuery(query string) *ParsedSearch {
	result := &ParsedSearch{
		IncludeTags: []string{},
		ExcludeTags: []string{},
	}

	query = strings.TrimSpace(query)

	// Handle special queries
	if query == "*" || query == "" {
		result.IsAllDocs = true
		return result
	}
	if query == "!untagged" {
		result.IsUntagged = true
		return result
	}

	// Find all tag references
	matches := tagPattern.FindAllStringSubmatch(query, -1)
	for _, match := range matches {
		prefix := match[1]
		tagName := match[2]
		if prefix == "#" {
			result.IncludeTags = append(result.IncludeTags, tagName)
		} else if prefix == "~" {
			result.ExcludeTags = append(result.ExcludeTags, tagName)
		}
	}

	// Remove tag patterns to get plain text
	textPart := tagPattern.ReplaceAllString(query, "")
	textPart = strings.TrimSpace(textPart)
	// Collapse multiple spaces
	textPart = regexp.MustCompile(`\s+`).ReplaceAllString(textPart, " ")

	result.TextTerms = textPart

	return result
}

// SavedSearchWithCount includes the document count for display
type SavedSearchWithCount struct {
	SavedSearch
	DocumentCount int `json:"document_count"`
}
