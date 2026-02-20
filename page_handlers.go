package main

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/drummonds/godocs/database"
	"github.com/drummonds/godocs/internal/build"
	"github.com/flosch/pongo2/v6"
	"github.com/labstack/echo/v4"
)

// DocumentWithMeta wraps a document with display-related metadata
type DocumentWithMeta struct {
	database.Document
	HasThumbnail bool
	Tags         []database.Tag
}

// HandleHomePage renders the home page with paginated documents.
// Default view is "collapsed" (stories + unstoried docs). Use ?view=flat for all docs.
func HandleHomePage(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		page := 1
		if p := c.QueryParam("page"); p != "" {
			if v, err := strconv.Atoi(p); err == nil && v > 0 {
				page = v
			}
		}
		pageSize := 20
		view := c.QueryParam("view")
		if view == "" {
			view = "collapsed"
		}
		showHidden := c.QueryParam("show_hidden") == "1"

		selectMode := c.QueryParam("select") == "1"

		ctx := pongo2.Context{
			"view":        view,
			"show_hidden": showHidden,
			"select_mode": selectMode,
		}

		if view == "flat" {
			documents, totalCount, err := tr.db.GetNewestDocumentsWithPagination(page, pageSize, showHidden)
			if err != nil {
				Logger.Error("Failed to fetch documents for home page", "error", err)
				documents = []database.Document{}
				totalCount = 0
			}
			docsWithMeta := tr.enrichDocuments(documents)
			totalPages := (totalCount + pageSize - 1) / pageSize

			ctx["documents"] = docsWithMeta
			ctx["page"] = page
			ctx["page_size"] = pageSize
			ctx["total_count"] = totalCount
			ctx["total_pages"] = totalPages
			ctx["has_next"] = page < totalPages
			ctx["has_previous"] = page > 1
			baseURL := "/?view=flat"
			if showHidden {
				baseURL += "&show_hidden=1"
			}
			ctx["base_url"] = baseURL
		} else {
			// Collapsed view: stories + unstoried documents
			stories, err := tr.db.GetStoriesWithMeta()
			if err != nil {
				Logger.Error("Failed to fetch stories", "error", err)
				stories = []database.StoryWithMeta{}
			}
			ctx["stories"] = stories

			documents, totalCount, err := tr.db.GetDocumentsWithoutStory(page, pageSize, showHidden)
			if err != nil {
				Logger.Error("Failed to fetch unstoried documents", "error", err)
				documents = []database.Document{}
				totalCount = 0
			}
			docsWithMeta := tr.enrichDocuments(documents)
			totalPages := (totalCount + pageSize - 1) / pageSize

			ctx["documents"] = docsWithMeta
			ctx["page"] = page
			ctx["page_size"] = pageSize
			ctx["total_count"] = totalCount
			ctx["total_pages"] = totalPages
			ctx["has_next"] = page < totalPages
			ctx["has_previous"] = page > 1
			baseURL := "/?view=collapsed"
			if showHidden {
				baseURL += "&show_hidden=1"
			}
			ctx["base_url"] = baseURL
		}

		return tr.Render(c, "home.html", ctx)
	}
}

// HandleSearchPage renders the search page with optional results
func HandleSearchPage(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		query := c.QueryParam("q")

		showHidden := c.QueryParam("show_hidden") == "1"
		selectMode := c.QueryParam("select") == "1"

		baseURL := "/search?q=" + query
		if showHidden {
			baseURL += "&show_hidden=1"
		}

		ctx := pongo2.Context{
			"query":       query,
			"base_url":    baseURL,
			"show_hidden": showHidden,
			"select_mode": selectMode,
		}

		// Load saved searches
		savedSearches, err := tr.db.GetAllSavedSearches()
		if err != nil {
			Logger.Error("Failed to load saved searches", "error", err)
			savedSearches = []database.SavedSearch{}
		}
		ctx["saved_searches"] = savedSearches

		if query != "" {
			page := 1
			if p := c.QueryParam("page"); p != "" {
				if v, err := strconv.Atoi(p); err == nil && v > 0 {
					page = v
				}
			}
			pageSize := 20

			parsed := database.ParseSearchQuery(query)
			documents, totalCount, err := tr.db.ExecuteSearch(parsed, page, pageSize, showHidden)
			if err != nil {
				Logger.Error("Search failed", "query", query, "error", err)
				documents = []database.Document{}
				totalCount = 0
			}

			docsWithMeta := tr.enrichDocuments(documents)
			totalPages := (totalCount + pageSize - 1) / pageSize

			ctx["documents"] = docsWithMeta
			ctx["total_count"] = totalCount
			ctx["page"] = page
			ctx["total_pages"] = totalPages
			ctx["has_next"] = page < totalPages
			ctx["has_previous"] = page > 1
		}

		return tr.Render(c, "search.html", ctx)
	}
}

// HandleCreateSavedSearch handles creating a new saved search from the search page
func HandleCreateSavedSearch(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		name := c.FormValue("name")
		query := c.FormValue("query")

		if name == "" || query == "" {
			return c.Redirect(http.StatusSeeOther, "/search")
		}

		now := time.Now()
		search := &database.SavedSearch{
			Name:      name,
			Query:     query,
			Icon:      c.FormValue("icon"),
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := tr.db.CreateSavedSearch(search); err != nil {
			Logger.Error("Failed to create saved search", "error", err)
		}

		return c.Redirect(http.StatusSeeOther, "/search")
	}
}

// HandleDeleteSavedSearch handles deleting a saved search
func HandleDeleteSavedSearch(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/search")
		}

		// Don't allow deleting system searches
		search, err := tr.db.GetSavedSearchByID(id)
		if err != nil || search == nil || search.IsSystem {
			return c.Redirect(http.StatusSeeOther, "/search")
		}

		if err := tr.db.DeleteSavedSearch(id); err != nil {
			Logger.Error("Failed to delete saved search", "id", id, "error", err)
		}

		return c.Redirect(http.StatusSeeOther, "/search")
	}
}

// HandleDocumentPage renders a single document view
func HandleDocumentPage(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		ulidStr := c.Param("ulid")

		document, httpStatus, err := database.FetchDocument(ulidStr, tr.db)
		if err != nil {
			if httpStatus == http.StatusNotFound {
				return tr.RenderWithStatus(c, "404.html", http.StatusNotFound, nil)
			}
			Logger.Error("Failed to fetch document", "ulid", ulidStr, "error", err)
			return tr.RenderWithStatus(c, "404.html", http.StatusNotFound, nil)
		}

		// Check for thumbnail
		hasThumbnail := tr.checkThumbnail(document.Path)

		// Get tags
		tags, err := tr.db.GetTagsForDocument(document.ID)
		if err != nil {
			Logger.Error("Failed to fetch tags for document", "error", err)
			tags = []database.Tag{}
		}

		// Prepare text excerpt (limit to first 2000 chars)
		textExcerpt := document.FullText
		if len(textExcerpt) > 2000 {
			textExcerpt = textExcerpt[:2000] + "..."
		}

		// Format document date for display
		documentDate := ""
		if document.DocumentDate != nil {
			documentDate = document.DocumentDate.Format("2006-01-02")
		}

		createdDate := ""
		if document.CreatedDate != nil {
			createdDate = document.CreatedDate.Format("2006-01-02 15:04")
		}
		updatedDate := ""
		if document.UpdatedDate != nil {
			updatedDate = document.UpdatedDate.Format("2006-01-02 15:04")
		}

		return tr.Render(c, "document.html", pongo2.Context{
			"doc":           document,
			"has_thumbnail": hasThumbnail,
			"tags":          tags,
			"text_excerpt":  textExcerpt,
			"document_date": documentDate,
			"created_date":  createdDate,
			"updated_date":  updatedDate,
		})
	}
}

// HandleDocumentEditPage renders the document edit page for tag management
func HandleDocumentEditPage(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		ulidStr := c.Param("ulid")

		document, httpStatus, err := database.FetchDocument(ulidStr, tr.db)
		if err != nil {
			if httpStatus == http.StatusNotFound {
				return tr.RenderWithStatus(c, "404.html", http.StatusNotFound, nil)
			}
			Logger.Error("Failed to fetch document", "ulid", ulidStr, "error", err)
			return tr.RenderWithStatus(c, "404.html", http.StatusNotFound, nil)
		}

		hasThumbnail := tr.checkThumbnail(document.Path)

		tags, err := tr.db.GetTagsForDocument(document.ID)
		if err != nil {
			Logger.Error("Failed to fetch tags for document", "error", err)
			tags = []database.Tag{}
		}

		allTags, err := tr.db.GetAllTags()
		if err != nil {
			Logger.Error("Failed to fetch all tags", "error", err)
			allTags = []database.Tag{}
		}

		// Filter out tags already on the document
		tagSet := make(map[int]bool, len(tags))
		for _, t := range tags {
			tagSet[t.ID] = true
		}
		availableTags := make([]database.Tag, 0, len(allTags))
		for _, t := range allTags {
			if !tagSet[t.ID] {
				availableTags = append(availableTags, t)
			}
		}

		// Format document date for HTML date input
		documentDateValue := ""
		if document.DocumentDate != nil {
			documentDateValue = document.DocumentDate.Format("2006-01-02")
		}

		createdDate := ""
		if document.CreatedDate != nil {
			createdDate = document.CreatedDate.Format("2006-01-02 15:04")
		}
		updatedDate := ""
		if document.UpdatedDate != nil {
			updatedDate = document.UpdatedDate.Format("2006-01-02 15:04")
		}

		return tr.Render(c, "document_edit.html", pongo2.Context{
			"doc":                 document,
			"has_thumbnail":       hasThumbnail,
			"tags":                tags,
			"available_tags":      availableTags,
			"document_date_value": documentDateValue,
			"created_date":        createdDate,
			"updated_date":        updatedDate,
		})
	}
}

// HandleDocumentAddTag adds a tag to a document
func HandleDocumentAddTag(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		ulidStr := c.Param("ulid")

		document, _, err := database.FetchDocument(ulidStr, tr.db)
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/")
		}

		tagID, err := strconv.Atoi(c.FormValue("tag_id"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/document/"+ulidStr+"/edit")
		}

		if err := tr.db.AddTagToDocument(document.ID, tagID); err != nil {
			Logger.Error("Failed to add tag to document", "docID", document.ID, "tagID", tagID, "error", err)
		}

		return c.Redirect(http.StatusSeeOther, "/document/"+ulidStr+"/edit")
	}
}

// HandleDocumentRemoveTag removes a tag from a document
func HandleDocumentRemoveTag(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		ulidStr := c.Param("ulid")

		document, _, err := database.FetchDocument(ulidStr, tr.db)
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/")
		}

		tagID, err := strconv.Atoi(c.Param("tagId"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/document/"+ulidStr+"/edit")
		}

		if err := tr.db.RemoveTagFromDocument(document.ID, tagID); err != nil {
			Logger.Error("Failed to remove tag from document", "docID", document.ID, "tagID", tagID, "error", err)
		}

		return c.Redirect(http.StatusSeeOther, "/document/"+ulidStr+"/edit")
	}
}

// HandleDocumentUpdateDate handles setting or clearing the document date
func HandleDocumentUpdateDate(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		ulidStr := c.Param("ulid")

		if c.FormValue("clear") == "1" {
			if err := tr.db.UpdateDocumentDate(ulidStr, nil); err != nil {
				Logger.Error("Failed to clear document date", "ulid", ulidStr, "error", err)
			}
			return c.Redirect(http.StatusSeeOther, "/document/"+ulidStr+"/edit")
		}

		dateStr := c.FormValue("document_date")
		if dateStr == "" {
			return c.Redirect(http.StatusSeeOther, "/document/"+ulidStr+"/edit")
		}

		parsed, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			Logger.Error("Failed to parse document date", "date", dateStr, "error", err)
			return c.Redirect(http.StatusSeeOther, "/document/"+ulidStr+"/edit")
		}

		if err := tr.db.UpdateDocumentDate(ulidStr, &parsed); err != nil {
			Logger.Error("Failed to update document date", "ulid", ulidStr, "error", err)
		}

		return c.Redirect(http.StatusSeeOther, "/document/"+ulidStr+"/edit")
	}
}

// HandleSearchHelpPage renders the search syntax help page
func HandleSearchHelpPage(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		return tr.Render(c, "search_help.html", nil)
	}
}

// HandleAboutPage renders the about/system info page
func HandleAboutPage(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		ocrConfigured := tr.config.TesseractPath != ""

		schemaVersion := "unknown"
		if v, err := tr.db.GetSchemaVersion(); err == nil {
			schemaVersion = v
		}

		logLevel := os.Getenv("LOG_LEVEL")
		if logLevel == "" {
			logLevel = "debug"
		}

		// Gather statistics
		documentCount := 0
		if _, total, err := tr.db.GetNewestDocumentsWithPagination(1, 1); err == nil {
			documentCount = total
		}
		tagCount := 0
		if tags, err := tr.db.GetAllTags(); err == nil {
			tagCount = len(tags)
		}
		untaggedCount := 0
		if _, total, err := tr.db.GetUntaggedDocuments(1, 1); err == nil {
			untaggedCount = total
		}

		return tr.Render(c, "about.html", pongo2.Context{
			"app_version":    build.GetVersion(),
			"schema_version": schemaVersion,
			"log_level":      logLevel,
			"ocr_configured": ocrConfigured,
			"ocr_path":       tr.config.TesseractPath,
			"database_type":  tr.config.DatabaseType,
			"database_host":  tr.config.DatabaseHost,
			"database_port":  tr.config.DatabasePort,
			"database_name":  tr.config.DatabaseDbname,
			"document_path":  tr.config.DocumentPath,
			"ingress_path":   tr.config.IngressPath,
			"document_count": documentCount,
			"tag_count":      tagCount,
			"untagged_count": untaggedCount,
		})
	}
}

// HandleNotFound renders the 404 page
func HandleNotFound(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		return tr.RenderWithStatus(c, "404.html", http.StatusNotFound, nil)
	}
}

// TagWithUsage wraps a Tag with its usage count for display
type TagWithUsage struct {
	database.Tag
	UsageCount int
}

// HandleTagsPage renders the tag manager page
func HandleTagsPage(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		tags, err := tr.db.GetAllTags()
		if err != nil {
			Logger.Error("Failed to fetch tags", "error", err)
			return tr.Render(c, "tags.html", pongo2.Context{
				"error": "Failed to load tags",
			})
		}

		tagsWithUsage := make([]TagWithUsage, len(tags))
		for i, tag := range tags {
			tagsWithUsage[i] = TagWithUsage{Tag: tag}
			count, err := tr.db.GetTagUsageCount(tag.ID)
			if err == nil {
				tagsWithUsage[i].UsageCount = count
			}
		}

		groups, err := tr.db.GetTagGroups()
		if err != nil {
			Logger.Error("Failed to fetch tag groups", "error", err)
			groups = []string{}
		}

		return tr.Render(c, "tags.html", pongo2.Context{
			"tags":       tagsWithUsage,
			"tag_groups": groups,
		})
	}
}

// HandleCreateTag handles creating a new tag
func HandleCreateTag(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		name := c.FormValue("name")
		color := c.FormValue("color")
		description := c.FormValue("description")
		tagGroup := c.FormValue("tag_group")

		tag := &database.Tag{
			Name:        name,
			Color:       color,
			Description: description,
		}
		if tagGroup != "" {
			tag.TagGroup = &tagGroup
		}

		if err := tr.db.CreateTag(tag); err != nil {
			Logger.Error("Failed to create tag", "error", err)
			return c.Redirect(http.StatusSeeOther, "/tags")
		}

		return c.Redirect(http.StatusSeeOther, "/tags")
	}
}

// HandleEditTagPage renders the tag edit form
func HandleEditTagPage(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/tags")
		}

		tag, err := tr.db.GetTagByID(id)
		if err != nil {
			Logger.Error("Failed to fetch tag", "id", id, "error", err)
			return c.Redirect(http.StatusSeeOther, "/tags")
		}

		tagGroupValue := ""
		if tag.TagGroup != nil {
			tagGroupValue = *tag.TagGroup
		}

		groups, err := tr.db.GetTagGroups()
		if err != nil {
			Logger.Error("Failed to fetch tag groups", "error", err)
			groups = []string{}
		}

		// Check if this tag is already a story
		isStory := false
		if tagGroupValue == "Story" {
			if s, _ := tr.db.GetStoryByTagID(tag.ID); s != nil {
				isStory = true
			}
		}

		return tr.Render(c, "tag_edit.html", pongo2.Context{
			"tag":             tag,
			"tag_group_value": tagGroupValue,
			"tag_groups":      groups,
			"is_story":        isStory,
		})
	}
}

// HandleUpdateTag handles updating an existing tag
func HandleUpdateTag(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/tags")
		}

		tag, err := tr.db.GetTagByID(id)
		if err != nil {
			Logger.Error("Failed to fetch tag for update", "id", id, "error", err)
			return c.Redirect(http.StatusSeeOther, "/tags")
		}

		tag.Name = c.FormValue("name")
		tag.Color = c.FormValue("color")
		tag.Description = c.FormValue("description")
		tagGroup := c.FormValue("tag_group")
		if tagGroup != "" {
			tag.TagGroup = &tagGroup
		} else {
			tag.TagGroup = nil
		}
		sortOrder, err := strconv.Atoi(c.FormValue("sort_order"))
		if err == nil {
			tag.SortOrder = sortOrder
		}

		if err := tr.db.UpdateTag(tag); err != nil {
			Logger.Error("Failed to update tag", "id", id, "error", err)
		}

		if tr.writeTagAliases != nil {
			if err := tr.writeTagAliases(tr.config.ConfigPath, tr.db); err != nil {
				Logger.Error("Failed to write tag aliases to config", "error", err)
			}
		}

		return c.Redirect(http.StatusSeeOther, "/tags")
	}
}

// HandleDeleteTag handles deleting a tag
func HandleDeleteTag(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.Redirect(http.StatusSeeOther, "/tags")
		}

		if err := tr.db.DeleteTag(id); err != nil {
			Logger.Error("Failed to delete tag", "id", id, "error", err)
		}

		return c.Redirect(http.StatusSeeOther, "/tags")
	}
}

// JobDisplay wraps a Job with formatted time strings for templates
type JobDisplay struct {
	database.Job
	StartedAtFmt   string
	CompletedAtFmt string
}

func formatJobTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

// HandleJobsPage renders the jobs manager page
func HandleJobsPage(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		jobs, err := tr.db.GetRecentJobs(50, 0)
		if err != nil {
			Logger.Error("Failed to fetch jobs", "error", err)
			return tr.Render(c, "jobs.html", pongo2.Context{
				"error": "Failed to load jobs",
			})
		}

		activeJobs, err := tr.db.GetActiveJobs()
		if err != nil {
			Logger.Error("Failed to fetch active jobs", "error", err)
			activeJobs = []database.Job{}
		}

		jobDisplays := make([]JobDisplay, len(jobs))
		for i, job := range jobs {
			jobDisplays[i] = JobDisplay{
				Job:            job,
				StartedAtFmt:   formatJobTime(job.StartedAt),
				CompletedAtFmt: formatJobTime(job.CompletedAt),
			}
		}

		return tr.Render(c, "jobs.html", pongo2.Context{
			"jobs":       jobDisplays,
			"has_active": len(activeJobs) > 0,
		})
	}
}

// HandleTriggerClean triggers a database cleanup job
func HandleTriggerClean(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		_, err := tr.runCleanupAsync()
		if err != nil {
			Logger.Error("Failed to trigger cleanup", "error", err)
		}
		return c.Redirect(http.StatusSeeOther, "/jobs")
	}
}

// HandleTriggerIngest triggers an ingestion job
func HandleTriggerIngest(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		_, err := tr.runIngestionAsync()
		if err != nil {
			Logger.Error("Failed to trigger ingestion", "error", err)
		}
		return c.Redirect(http.StatusSeeOther, "/jobs")
	}
}

// enrichDocuments adds thumbnail and tag info to documents for display
func (tr *TemplateRenderer) enrichDocuments(docs []database.Document) []DocumentWithMeta {
	result := make([]DocumentWithMeta, len(docs))
	for i, doc := range docs {
		result[i] = DocumentWithMeta{
			Document:     doc,
			HasThumbnail: tr.checkThumbnail(doc.Path),
		}
		tags, err := tr.db.GetTagsForDocument(doc.ID)
		if err != nil {
			Logger.Error("Failed to fetch tags for document in enrichDocuments",
				"docID", doc.ID, "docName", doc.Name, "error", err)
		} else {
			result[i].Tags = tags
		}
	}
	return result
}
