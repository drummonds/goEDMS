package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"codeberg.org/hum3/godocs/database"
	"codeberg.org/hum3/godocs/engine"
	"github.com/flosch/pongo2/v6"
	"github.com/labstack/echo/v4"
)

// BulkDocInfo holds minimal document info for the bulk edit page
type BulkDocInfo struct {
	ULID         string
	Name         string
	HasThumbnail bool
}

// HandleBulkEditPage renders the bulk edit page for multiple documents.
// Accepts ULIDs via POST form fields or GET query param.
func HandleBulkEditPage(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		ulids := parseULIDs(c)
		if len(ulids) == 0 {
			return c.Redirect(http.StatusSeeOther, "/")
		}

		// Fetch documents and compute common tags
		var docs []BulkDocInfo
		tagCounts := map[int]int{}
		var allDocTags []database.Tag

		for _, ulidStr := range ulids {
			doc, _, err := database.FetchDocument(ulidStr, tr.db)
			if err != nil {
				continue
			}
			docs = append(docs, BulkDocInfo{
				ULID:         ulidStr,
				Name:         doc.Name,
				HasThumbnail: tr.checkThumbnail(doc.Path),
			})
			tags, err := tr.db.GetTagsForDocument(doc.ID)
			if err != nil {
				continue
			}
			for _, t := range tags {
				tagCounts[t.ID]++
				// Keep one copy of the tag for display
				found := false
				for _, existing := range allDocTags {
					if existing.ID == t.ID {
						found = true
						break
					}
				}
				if !found {
					allDocTags = append(allDocTags, t)
				}
			}
		}

		// Common tags = present on ALL selected documents
		var commonTags []database.Tag
		commonTagSet := map[int]bool{}
		for _, t := range allDocTags {
			if tagCounts[t.ID] == len(docs) {
				commonTags = append(commonTags, t)
				commonTagSet[t.ID] = true
			}
		}

		// Available tags (not already common to all)
		allTags, err := tr.db.GetAllTags()
		if err != nil {
			allTags = []database.Tag{}
		}
		var availableTags []database.Tag
		for _, t := range allTags {
			if !commonTagSet[t.ID] {
				availableTags = append(availableTags, t)
			}
		}

		// Stories
		stories, err := tr.db.GetAllStories()
		if err != nil {
			stories = []database.Story{}
		}

		ulidsJoined := strings.Join(ulids, ",")

		// Look up the "Hide" tag for the quick-hide button
		var hideTagID int
		if hideTag, err := tr.db.GetTagByName("Hide"); err == nil && hideTag != nil {
			hideTagID = hideTag.ID
		}

		return tr.Render(c, "bulk_edit.html", pongo2.Context{
			"docs":           docs,
			"ulids":          ulidsJoined,
			"common_tags":    commonTags,
			"available_tags": availableTags,
			"stories":        stories,
			"doc_count":      len(docs),
			"hide_tag_id":    hideTagID,
		})
	}
}

// HandleBulkAddTag adds a tag to all selected documents.
func HandleBulkAddTag(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		ulids := parseULIDs(c)
		tagID, err := strconv.Atoi(c.FormValue("tag_id"))
		if err != nil {
			return redirectToBulkEdit(c, ulids)
		}

		for _, ulidStr := range ulids {
			doc, _, err := database.FetchDocument(ulidStr, tr.db)
			if err != nil {
				continue
			}
			if err := tr.db.AddTagToDocument(doc.ID, tagID); err != nil {
				Logger.Error("Bulk add tag failed", "docID", doc.ID, "tagID", tagID, "error", err)
			}
		}

		return redirectToBulkEdit(c, ulids)
	}
}

// HandleBulkRemoveTag removes a tag from all selected documents.
func HandleBulkRemoveTag(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		ulids := parseULIDs(c)
		tagID, err := strconv.Atoi(c.Param("tagId"))
		if err != nil {
			return redirectToBulkEdit(c, ulids)
		}

		for _, ulidStr := range ulids {
			doc, _, err := database.FetchDocument(ulidStr, tr.db)
			if err != nil {
				continue
			}
			if err := tr.db.RemoveTagFromDocument(doc.ID, tagID); err != nil {
				Logger.Error("Bulk remove tag failed", "docID", doc.ID, "tagID", tagID, "error", err)
			}
		}

		return redirectToBulkEdit(c, ulids)
	}
}

// HandleBulkAddToStory adds all selected documents to a story.
func HandleBulkAddToStory(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		ulids := parseULIDs(c)
		storyID, err := strconv.Atoi(c.FormValue("story_id"))
		if err != nil {
			return redirectToBulkEdit(c, ulids)
		}

		for _, ulidStr := range ulids {
			doc, _, err := database.FetchDocument(ulidStr, tr.db)
			if err != nil {
				continue
			}
			if err := tr.db.AddDocumentToStory(doc.ID, storyID); err != nil {
				Logger.Error("Bulk add to story failed", "docID", doc.ID, "storyID", storyID, "error", err)
			}
		}

		return redirectToBulkEdit(c, ulids)
	}
}

// HandleBulkSetDate sets a date field on all selected documents.
func HandleBulkSetDate(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		ulids := parseULIDs(c)
		field := c.FormValue("field")
		dateStr := c.FormValue("date")

		if dateStr == "" || (field != "created_date" && field != "updated_date" && field != "document_date") {
			return redirectToBulkEdit(c, ulids)
		}

		parsed, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			Logger.Error("Failed to parse bulk date", "date", dateStr, "error", err)
			return redirectToBulkEdit(c, ulids)
		}

		for _, ulidStr := range ulids {
			switch field {
			case "document_date":
				if err := tr.db.UpdateDocumentDate(ulidStr, &parsed); err != nil {
					Logger.Error("Bulk set document_date failed", "ulid", ulidStr, "error", err)
				}
			case "created_date":
				meta := database.DocumentMetadataUpdate{CreatedDate: &parsed}
				if err := tr.db.UpdateDocumentMetadata(ulidStr, meta); err != nil {
					Logger.Error("Bulk set created_date failed", "ulid", ulidStr, "error", err)
				}
			case "updated_date":
				meta := database.DocumentMetadataUpdate{UpdatedDate: &parsed}
				if err := tr.db.UpdateDocumentMetadata(ulidStr, meta); err != nil {
					Logger.Error("Bulk set updated_date failed", "ulid", ulidStr, "error", err)
				}
			}
		}

		return redirectToBulkEdit(c, ulids)
	}
}

// HandleBulkArchiveDocuments archives all selected documents using engine.ArchiveDocument.
func HandleBulkArchiveDocuments(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		ulids := parseULIDs(c)
		if len(ulids) == 0 {
			return c.Redirect(http.StatusSeeOther, "/")
		}

		for _, ulidStr := range ulids {
			doc, _, err := database.FetchDocument(ulidStr, tr.db)
			if err != nil {
				Logger.Error("Archive: doc not found", "ulid", ulidStr, "error", err)
				continue
			}
			if doc.ArchiveStatus != nil {
				continue // already archived
			}
			if err := engine.ArchiveDocument(doc, tr.db, tr.config.DocumentPath, tr.config.ArchivePath, "user-initiated"); err != nil {
				Logger.Error("Archive failed", "ulid", ulidStr, "error", err)
			}
		}

		return c.Redirect(http.StatusSeeOther, "/")
	}
}

// parseULIDs extracts ULIDs from POST form or GET query. Supports both
// multi-valued "ulids" fields (from checkboxes) and comma-separated values.
func parseULIDs(c echo.Context) []string {
	// Try POST form first
	if err := c.Request().ParseForm(); err == nil {
		if vals := c.Request().PostForm["ulids"]; len(vals) > 0 {
			return deduplicateULIDs(vals)
		}
	}
	// Fall back to query param
	if q := c.QueryParam("ulids"); q != "" {
		return deduplicateULIDs([]string{q})
	}
	return nil
}

func deduplicateULIDs(values []string) []string {
	// If we got a single comma-separated value, split it
	if len(values) == 1 && strings.Contains(values[0], ",") {
		values = strings.Split(values[0], ",")
	}
	// Deduplicate and trim
	seen := map[string]bool{}
	var result []string
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// redirectToBulkEdit POSTs back to the bulk edit page preserving ULID selection.
// Since we can't POST via redirect, we use a GET-after-POST pattern with a
// self-submitting form. Instead we just do an internal render.
func redirectToBulkEdit(c echo.Context, ulids []string) error {
	// We need to re-POST to the bulk edit page. Use 303 See Other which
	// browsers convert to GET. But our bulk edit page expects POST.
	// So we'll set the ulids as a query parameter for a GET fallback.
	return c.Redirect(http.StatusSeeOther, "/documents/bulk-edit?ulids="+strings.Join(ulids, ","))
}
