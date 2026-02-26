package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/drummonds/godocs/database"
	"github.com/drummonds/godocs/engine"
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

		return tr.Render(c, "bulk_edit.html", pongo2.Context{
			"docs":           docs,
			"ulids":          ulidsJoined,
			"common_tags":    commonTags,
			"available_tags": availableTags,
			"stories":        stories,
			"doc_count":      len(docs),
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

// lifecycleMetadata is written to .lifecycle.json at archive time.
type lifecycleMetadata struct {
	ArchivedAt    string `json:"archived_at"`
	ArchivedBy    string `json:"archived_by"`
	ArchiveReason string `json:"archive_reason"`
	OriginalPath  string `json:"original_path"`
	Hash          string `json:"hash"`
	ULID          string `json:"ulid"`
	DBID          int    `json:"db_id"`
	SchemaVersion string `json:"schema_version"`
}

// HandleBulkArchiveDocuments archives all selected documents:
// sets archive_status='pending', adds Archive Pending tag, exports .tags.json,
// writes .lifecycle.json, moves files to archive folder, updates DB path.
func HandleBulkArchiveDocuments(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		ulids := parseULIDs(c)
		if len(ulids) == 0 {
			return c.Redirect(http.StatusSeeOther, "/")
		}

		archivePath := tr.config.ArchivePath
		documentPath := tr.config.DocumentPath

		// Find the "Archive Pending" system tag
		archiveTag, err := tr.db.GetTagByName("Archive Pending")
		if err != nil || archiveTag == nil {
			Logger.Error("Archive Pending tag not found — run migration")
			return c.Redirect(http.StatusSeeOther, "/")
		}

		now := time.Now()
		pending := "pending"

		for _, ulidStr := range ulids {
			doc, _, err := database.FetchDocument(ulidStr, tr.db)
			if err != nil {
				Logger.Error("Archive: doc not found", "ulid", ulidStr, "error", err)
				continue
			}
			if doc.ArchiveStatus != nil {
				continue // already archived
			}

			// 1. Set archive_status='pending', archived_at=now
			if err := tr.db.UpdateDocumentArchiveStatus(ulidStr, &pending, &now); err != nil {
				Logger.Error("Archive: failed to set status", "ulid", ulidStr, "error", err)
				continue
			}

			// 2. Add Archive Pending tag
			tr.db.AddTagToDocument(doc.ID, archiveTag.ID)

			// 3. Export final .tags.json
			exportTagsSidecar(&doc, tr.db)

			// 4. Write .lifecycle.json sidecar (at current doc path, before moving)
			lifecycle := lifecycleMetadata{
				ArchivedAt:    now.UTC().Format(time.RFC3339),
				ArchivedBy:    "godocs",
				ArchiveReason: "user-initiated",
				OriginalPath:  doc.Path,
				Hash:          doc.Hash,
				ULID:          ulidStr,
				DBID:          doc.ID,
				SchemaVersion: "1",
			}
			lifecyclePath := engine.GetLifecyclePath(doc.Path)
			writeLifecycleJSON(lifecyclePath, &lifecycle)

			// 5. Move all files to archive folder
			// Compute archive destination by replacing documentPath prefix with archivePath
			relPath, err := filepath.Rel(documentPath, doc.Path)
			if err != nil {
				Logger.Error("Archive: cannot compute relative path", "path", doc.Path, "error", err)
				continue
			}
			archiveDocPath := filepath.ToSlash(filepath.Join(archivePath, relPath))
			archiveDir := filepath.Dir(archiveDocPath)

			if err := os.MkdirAll(archiveDir, 0755); err != nil {
				Logger.Error("Archive: cannot create archive dir", "dir", archiveDir, "error", err)
				continue
			}

			// Move the original document and all sidecars
			sidecarBase := engine.SidecarBasePath(doc.Path)
			archiveSidecarBase := engine.SidecarBasePath(archiveDocPath)
			filesToMove := []struct{ src, dst string }{
				{doc.Path, archiveDocPath},
				{sidecarBase + ".ocr.txt", archiveSidecarBase + ".ocr.txt"},
				{sidecarBase + ".thumb.png", archiveSidecarBase + ".thumb.png"},
				{sidecarBase + ".tags.json", archiveSidecarBase + ".tags.json"},
				{sidecarBase + ".lifecycle.json", archiveSidecarBase + ".lifecycle.json"},
			}

			allMoved := true
			for _, f := range filesToMove {
				if _, err := os.Stat(f.src); os.IsNotExist(err) {
					continue // sidecar may not exist
				}
				if err := os.Rename(f.src, f.dst); err != nil {
					Logger.Error("Archive: failed to move file", "src", f.src, "dst", f.dst, "error", err)
					allMoved = false
				}
			}

			if !allMoved {
				Logger.Warn("Archive: some files failed to move", "ulid", ulidStr)
			}

			// 6. Update DB path and folder to archive location
			archiveFolder := filepath.ToSlash(filepath.Dir(archiveDocPath))
			if err := tr.db.UpdateDocumentPath(ulidStr, archiveDocPath, archiveFolder); err != nil {
				Logger.Error("Archive: failed to update path", "ulid", ulidStr, "error", err)
			}
		}

		return c.Redirect(http.StatusSeeOther, "/")
	}
}

// exportTagsSidecar exports tags for a document to its .tags.json sidecar.
func exportTagsSidecar(doc *database.Document, db database.Repository) {
	tags, err := db.GetTagsForDocument(doc.ID)
	if err != nil {
		Logger.Error("Export tags: failed to get tags", "docID", doc.ID, "error", err)
		return
	}

	tagData := &database.DocumentTagsAndDimensions{
		Tags:      []string{},
		TagGroups: make(map[string]string),
	}
	for _, tag := range tags {
		if tag.TagGroup != nil && *tag.TagGroup != "" {
			tagData.TagGroups[*tag.TagGroup] = tag.Name
		} else {
			tagData.Tags = append(tagData.Tags, tag.Name)
		}
	}

	tagsPath := engine.SidecarBasePath(doc.Path) + ".tags.json"
	if err := os.MkdirAll(filepath.Dir(tagsPath), 0755); err != nil {
		Logger.Error("Export tags: mkdir failed", "path", tagsPath, "error", err)
		return
	}
	data, err := json.MarshalIndent(tagData, "", "  ")
	if err != nil {
		Logger.Error("Export tags: marshal failed", "error", err)
		return
	}
	if err := os.WriteFile(tagsPath, data, 0644); err != nil {
		Logger.Error("Export tags: write failed", "path", tagsPath, "error", err)
	}
}

// writeLifecycleJSON writes the .lifecycle.json sidecar file.
func writeLifecycleJSON(path string, meta *lifecycleMetadata) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		Logger.Error("Lifecycle: mkdir failed", "path", path, "error", err)
		return
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		Logger.Error("Lifecycle: marshal failed", "error", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		Logger.Error("Lifecycle: write failed", "path", path, "error", err)
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
