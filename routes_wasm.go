//go:build js && wasm

package main

import (
	"github.com/labstack/echo/v4"
)

// registerWASMRoutes registers all routes for the WASM demo.
// Includes all SSR page routes and read-only API endpoints.
// Excludes: file upload, delete, move, ingest, clean, filesystem, swagger docs.
func registerWASMRoutes(e *echo.Echo, tr *TemplateRenderer) {
	// --- SSR page routes ---
	e.GET("/", HandleHomePage(tr))
	e.GET("/search", HandleSearchPage(tr))
	e.POST("/search/saved", HandleCreateSavedSearch(tr))
	e.POST("/search/saved/:id/delete", HandleDeleteSavedSearch(tr))
	e.GET("/search/help", HandleSearchHelpPage(tr))
	e.GET("/document/:ulid", HandleDocumentPage(tr))
	e.GET("/document/:ulid/edit", HandleDocumentEditPage(tr))
	e.POST("/document/:ulid/date", HandleDocumentUpdateDate(tr))
	e.POST("/document/:ulid/tags", HandleDocumentAddTag(tr))
	e.POST("/document/:ulid/tags/:tagId/remove", HandleDocumentRemoveTag(tr))
	e.GET("/about", HandleAboutPage(tr))

	// Bulk edit routes
	e.POST("/documents/bulk-edit", HandleBulkEditPage(tr))
	e.GET("/documents/bulk-edit", HandleBulkEditPage(tr))
	e.POST("/documents/bulk-edit/tags", HandleBulkAddTag(tr))
	e.POST("/documents/bulk-edit/tags/:tagId/remove", HandleBulkRemoveTag(tr))
	e.POST("/documents/bulk-edit/story", HandleBulkAddToStory(tr))
	e.POST("/documents/bulk-edit/date", HandleBulkSetDate(tr))

	// Tag management routes
	e.GET("/tags", HandleTagsPage(tr))
	e.POST("/tags", HandleCreateTag(tr))
	e.GET("/tags/:id/edit", HandleEditTagPage(tr))
	e.POST("/tags/:id", HandleUpdateTag(tr))
	e.POST("/tags/:id/delete", HandleDeleteTag(tr))
	e.POST("/tags/:id/to-story", HandleConvertTagToStory(tr))

	// Story management routes
	e.GET("/stories", HandleStoriesPage(tr))
	e.GET("/stories/new", HandleNewStoryPage(tr))
	e.POST("/stories", HandleCreateStory(tr))
	e.GET("/stories/:id/edit", HandleEditStoryPage(tr))
	e.POST("/stories/:id", HandleUpdateStory(tr))
	e.POST("/stories/:id/delete", HandleDeleteStory(tr))
	e.POST("/stories/:id/to-tag", HandleConvertStoryToTag(tr))
	e.POST("/stories/:id/tags", HandleAddStoryTag(tr))
	e.POST("/stories/:id/tags/:tagId/remove", HandleRemoveStoryTag(tr))
	e.POST("/stories/:id/documents", HandleAddDocumentToStory(tr))
	e.POST("/stories/:id/documents/:ulid/remove", HandleRemoveDocumentFromStory(tr))

	// Jobs management routes (read-only in demo)
	e.GET("/jobs", HandleJobsPage(tr))
	e.POST("/jobs/clean", HandleTriggerClean(tr))
	e.POST("/jobs/ingest", HandleTriggerIngest(tr))

	// --- Placeholder routes for document viewing/thumbnails ---
	e.GET("/api/document/:id/thumbnail", serveEmbedThumbnail())
	e.GET("/document/view/:ulid", serveEmbedDocument())
}
