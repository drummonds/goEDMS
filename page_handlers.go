package main

import (
	"net/http"
	"os"
	"path/filepath"
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

// HandleHomePage renders the home page with paginated documents
func HandleHomePage(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		page := 1
		if p := c.QueryParam("page"); p != "" {
			if v, err := strconv.Atoi(p); err == nil && v > 0 {
				page = v
			}
		}
		pageSize := 20

		documents, totalCount, err := tr.db.GetNewestDocumentsWithPagination(page, pageSize)
		if err != nil {
			Logger.Error("Failed to fetch documents for home page", "error", err)
			documents = []database.Document{}
			totalCount = 0
		}

		// Enrich documents with thumbnails and tags
		docsWithMeta := enrichDocuments(documents, tr.db)

		totalPages := (totalCount + pageSize - 1) / pageSize

		return tr.Render(c, "home.html", pongo2.Context{
			"documents":    docsWithMeta,
			"page":         page,
			"page_size":    pageSize,
			"total_count":  totalCount,
			"total_pages":  totalPages,
			"has_next":     page < totalPages,
			"has_previous": page > 1,
			"base_url":     "/",
		})
	}
}

// HandleSearchPage renders the search page with optional results
func HandleSearchPage(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		query := c.QueryParam("q")

		ctx := pongo2.Context{
			"query":    query,
			"base_url": "/search",
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
			documents, totalCount, err := tr.db.ExecuteSearch(parsed, page, pageSize)
			if err != nil {
				Logger.Error("Search failed", "query", query, "error", err)
				documents = []database.Document{}
				totalCount = 0
			}

			docsWithMeta := enrichDocuments(documents, tr.db)
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
		hasThumbnail := checkThumbnailExists(document.Path)

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

		return tr.Render(c, "document.html", pongo2.Context{
			"doc":           document,
			"has_thumbnail": hasThumbnail,
			"tags":          tags,
			"text_excerpt":  textExcerpt,
		})
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

		return tr.Render(c, "tags.html", pongo2.Context{
			"tags": tagsWithUsage,
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

		return tr.Render(c, "tag_edit.html", pongo2.Context{
			"tag":             tag,
			"tag_group_value": tagGroupValue,
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
		_, err := tr.engine.RunCleanupAsync()
		if err != nil {
			Logger.Error("Failed to trigger cleanup", "error", err)
		}
		return c.Redirect(http.StatusSeeOther, "/jobs")
	}
}

// HandleTriggerIngest triggers an ingestion job
func HandleTriggerIngest(tr *TemplateRenderer) echo.HandlerFunc {
	return func(c echo.Context) error {
		_, err := tr.engine.RunIngestionAsync()
		if err != nil {
			Logger.Error("Failed to trigger ingestion", "error", err)
		}
		return c.Redirect(http.StatusSeeOther, "/jobs")
	}
}

// enrichDocuments adds thumbnail and tag info to documents for display
func enrichDocuments(docs []database.Document, db database.Repository) []DocumentWithMeta {
	result := make([]DocumentWithMeta, len(docs))
	for i, doc := range docs {
		result[i] = DocumentWithMeta{
			Document:     doc,
			HasThumbnail: checkThumbnailExists(doc.Path),
		}
		tags, err := db.GetTagsForDocument(doc.ID)
		if err == nil {
			result[i].Tags = tags
		}
	}
	return result
}

// checkThumbnailExists checks if a thumbnail file exists for a document
func checkThumbnailExists(docPath string) bool {
	ext := filepath.Ext(docPath)
	if ext == "" {
		return false
	}
	thumbnailPath := docPath[:len(docPath)-len(ext)] + ".tn_256.png"
	_, err := os.Stat(thumbnailPath)
	return err == nil
}
