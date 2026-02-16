package main

import (
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	config "github.com/drummonds/godocs/config"
	database "github.com/drummonds/godocs/database"
	engine "github.com/drummonds/godocs/engine"
	"github.com/drummonds/godocs/internal/build"
	"github.com/drummonds/godocs/internal/docs"
	"github.com/drummonds/lofigui"
)

//go:embed public/built/favicon.ico
var publicFS embed.FS

//go:embed docs/swagger.json docs/swagger.yaml docs/openapi.yaml
var docsFS embed.FS

//go:embed static/swagger-ui/*
var swaggerUIFS embed.FS

// Logger is global since we will need it everywhere
var Logger *slog.Logger

// injectGlobals injects all of our globals into their packages
func injectGlobals(logger *slog.Logger) {
	Logger = logger
	database.Logger = Logger
	config.Logger = Logger
	engine.Logger = Logger
}

func main() {
	serverConfig, logger := config.SetupServer()
	injectGlobals(logger)

	// Log version information
	version := build.GetVersion()
	logger.Info("Starting godocs", "version", version)
	fmt.Printf("\ngodocs version %s\n", version)

	// Show info banner if using ephemeral database
	if serverConfig.DatabaseType == "ephemeral" {
		fmt.Println("\n" + strings.Repeat("=", 50))
		fmt.Println("EPHEMERAL DATABASE MODE")
		fmt.Println(strings.Repeat("=", 50))
		fmt.Println("Database will be destroyed on exit")
		fmt.Println("Perfect for testing and development")
		fmt.Println(strings.Repeat("=", 50) + "\n")
	}

	// Setup database
	Logger.Info("Setting up database", "type", serverConfig.DatabaseType)
	db := database.NewRepository(serverConfig)
	defer db.Close()

	Logger.Info("Database setup complete")
	database.WriteConfigToDB(serverConfig, db)
	Logger.Info("Config written to DB")

	e := echo.New()
	e.HideBanner = true

	// Create lofigui app for polling support
	app := lofigui.NewApp()
	app.Version = version

	serverHandler := engine.ServerHandler{DB: db, Echo: e, ServerConfig: serverConfig}

	// Create template renderer
	tr := NewTemplateRenderer(app, db, serverConfig, &serverHandler)

	// Custom error handler
	e.HTTPErrorHandler = createHTTPErrorHandler(e, tr)
	docsHandler := docs.DocsHandler{DocsFS: docsFS, SwaggerUIFS: swaggerUIFS}

	Logger.Info("About to initialize schedules")
	serverHandler.InitializeSchedules(db)
	Logger.Info("Schedules initialized, about to run startup checks")
	serverHandler.StartupChecks()
	Logger.Info("Startup checks complete")

	// Request logging middleware
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()

			start := time.Now()
			err := next(c)
			latency := time.Since(start)

			if err != nil {
				Logger.Error("Request handler error",
					"method", req.Method,
					"path", req.URL.Path,
					"error", err,
					"latency", latency.String(),
				)
			} else if res.Status >= 500 {
				Logger.Error("HTTP 5xx response",
					"method", req.Method,
					"path", req.URL.Path,
					"status", res.Status,
					"latency", latency.String(),
				)
			} else if res.Status == http.StatusNotFound && strings.HasPrefix(req.URL.Path, "/api/") {
				Logger.Warn("API endpoint not found",
					"method", req.Method,
					"path", req.URL.Path,
					"status", res.Status,
					"latency", latency.String(),
					"ip", c.RealIP(),
				)
			} else {
				Logger.Debug("HTTP request",
					"method", req.Method,
					"path", req.URL.Path,
					"status", res.Status,
					"latency", latency.String(),
				)
			}

			return err
		}
	})

	// --- Static assets ---

	// Serve favicon
	e.GET("/favicon.ico", func(c echo.Context) error {
		data, err := publicFS.ReadFile("public/built/favicon.ico")
		if err != nil {
			return c.String(http.StatusNotFound, "favicon.ico not found")
		}
		return c.Blob(http.StatusOK, "image/x-icon", data)
	})

	// --- Swagger/API documentation ---
	e.GET("/api/docs/swagger.json", docsHandler.GetSwaggerJSON)
	e.GET("/api/docs/openapi.yaml", docsHandler.GetOpenAPIYAML)
	e.GET("/api/docs/swagger-ui.css", docsHandler.GetSwaggerUICSS)
	e.GET("/api/docs/swagger-ui-bundle.js", docsHandler.GetSwaggerUIBundle)
	e.GET("/api/docs/swagger-ui-standalone-preset.js", docsHandler.GetSwaggerUIPreset)
	e.GET("/api/docs", docsHandler.GetSwaggerUI)

	// --- API routes (all preserved) ---

	// Document API routes
	e.GET("/api/documents/latest", serverHandler.GetLatestDocuments)
	e.GET("/api/documents/filesystem", serverHandler.GetDocumentFileSystem)
	e.GET("/api/document/:id", serverHandler.GetDocument)
	e.GET("/api/document/:id/thumbnail", serverHandler.GetDocumentThumbnail)
	e.GET("/api/document/:id/status", serverHandler.GetDocumentStatus)
	e.GET("/api/document/:id/text", serverHandler.GetDocumentText)
	e.POST("/api/document/:id/thumbnail/regenerate", serverHandler.RegenerateThumbnail)
	e.PUT("/api/document/:id/text", serverHandler.UpdateDocumentText)
	e.PUT("/api/document/:id/date", serverHandler.UpdateDocumentDate)
	e.PUT("/api/document/:id/metadata", serverHandler.UpdateDocumentMetadata)
	e.DELETE("/api/document/*", serverHandler.DeleteFile)
	e.PATCH("/api/document/move/*", serverHandler.MoveDocuments)
	e.POST("/api/document/upload", serverHandler.UploadDocuments)

	// Document view route (serves actual files)
	e.GET("/document/view/:ulid", serverHandler.ViewDocument)

	// Folder API routes
	e.GET("/api/folder/:folder", serverHandler.GetFolder)
	e.POST("/api/folder/*", serverHandler.CreateFolder)

	// Search API routes
	e.GET("/api/search", serverHandler.SearchDocuments)
	e.POST("/api/search/reindex", serverHandler.ReindexSearchDocuments)

	// Admin API routes
	e.POST("/api/ingest", serverHandler.RunIngestNow)
	e.POST("/api/clean", serverHandler.CleanDatabase)
	e.GET("/api/about", serverHandler.GetAboutInfo)
	e.POST("/api/log", serverHandler.LogFromFrontend)

	// Word cloud API routes
	e.GET("/api/wordcloud", serverHandler.GetWordCloud)
	e.POST("/api/wordcloud/recalculate", serverHandler.RecalculateWordCloud)

	// Job tracking API routes
	e.GET("/api/jobs", serverHandler.GetRecentJobs)
	e.GET("/api/jobs/active", serverHandler.GetActiveJobs)

	// Tag API routes
	e.GET("/api/tags", serverHandler.GetAllTags)
	e.GET("/api/tags/usage", serverHandler.GetAllTagsWithUsage)
	e.GET("/api/tags/groups", serverHandler.GetTagGroups)
	e.POST("/api/tags", serverHandler.CreateTag)
	e.PUT("/api/tags/:id", serverHandler.UpdateTag)
	e.DELETE("/api/tags/:id", serverHandler.DeleteTag)

	// Document tag API routes
	e.GET("/api/documents/:ulid/tags", serverHandler.GetDocumentTags)
	e.POST("/api/documents/:ulid/tags", serverHandler.AddDocumentTag)
	e.DELETE("/api/documents/:ulid/tags/:tagId", serverHandler.RemoveDocumentTag)

	// Dimension API routes
	e.GET("/api/dimensions", serverHandler.GetAllDimensions)
	e.GET("/api/documents/:ulid/dimensions", serverHandler.GetDocumentDimensions)
	e.POST("/api/documents/:ulid/dimensions", serverHandler.SetDocumentDimension)
	e.DELETE("/api/documents/:ulid/dimensions/:dimensionName", serverHandler.RemoveDocumentDimension)
	e.GET("/api/jobs/:id", serverHandler.GetJob)

	// Saved search API routes
	e.GET("/api/saved-searches", serverHandler.GetAllSavedSearches)
	e.GET("/api/saved-searches/:id", serverHandler.GetSavedSearch)
	e.POST("/api/saved-searches", serverHandler.CreateSavedSearch)
	e.PUT("/api/saved-searches/:id", serverHandler.UpdateSavedSearch)
	e.DELETE("/api/saved-searches/:id", serverHandler.DeleteSavedSearch)
	e.GET("/api/saved-searches/:id/execute", serverHandler.ExecuteSavedSearch)

	// Search execution routes
	e.GET("/api/search/query", serverHandler.ExecuteAdHocSearch)
	e.GET("/api/documents/by-tag/:tagId", serverHandler.GetDocumentsByTag)
	e.GET("/api/documents/untagged", serverHandler.GetUntaggedDocuments)

	// --- HTML page routes (SSR) ---
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

	// Tag management routes
	e.GET("/tags", HandleTagsPage(tr))
	e.POST("/tags", HandleCreateTag(tr))
	e.GET("/tags/:id/edit", HandleEditTagPage(tr))
	e.POST("/tags/:id", HandleUpdateTag(tr))
	e.POST("/tags/:id/delete", HandleDeleteTag(tr))

	// Story management routes
	e.GET("/stories", HandleStoriesPage(tr))
	e.GET("/stories/new", HandleNewStoryPage(tr))
	e.POST("/stories", HandleCreateStory(tr))
	e.GET("/stories/:id/edit", HandleEditStoryPage(tr))
	e.POST("/stories/:id", HandleUpdateStory(tr))
	e.POST("/stories/:id/delete", HandleDeleteStory(tr))
	e.POST("/stories/:id/tags", HandleAddStoryTag(tr))
	e.POST("/stories/:id/tags/:tagId/remove", HandleRemoveStoryTag(tr))
	e.POST("/stories/:id/documents", HandleAddDocumentToStory(tr))
	e.POST("/stories/:id/documents/:ulid/remove", HandleRemoveDocumentFromStory(tr))

	// Jobs management routes
	e.GET("/jobs", HandleJobsPage(tr))
	e.POST("/jobs/clean", HandleTriggerClean(tr))
	e.POST("/jobs/ingest", HandleTriggerIngest(tr))

	// --- Start server ---
	displayAddr := serverConfig.ListenAddrIP
	if displayAddr == "" {
		displayAddr = "localhost"
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Printf("godocs server: http://%s:%s\n", displayAddr, serverConfig.ListenAddrPort)
	fmt.Printf("API docs:      http://%s:%s/api/docs\n", displayAddr, serverConfig.ListenAddrPort)
	fmt.Println(strings.Repeat("=", 50) + "\n")

	addr := fmt.Sprintf("%s:%s", serverConfig.ListenAddrIP, serverConfig.ListenAddrPort)
	Logger.Info("Starting server", "address", addr)

	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		Logger.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

// createHTTPErrorHandler returns a custom HTTP error handler
func createHTTPErrorHandler(e *echo.Echo, tr *TemplateRenderer) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		code := http.StatusInternalServerError
		message := "Internal Server Error"
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			if msg, ok := he.Message.(string); ok {
				message = msg
			}
		} else {
			message = err.Error()
		}

		// Log all errors
		Logger.Error("HTTP error",
			"status", code,
			"error", err,
			"method", c.Request().Method,
			"path", c.Request().URL.Path,
		)

		// Don't try to write if response is already committed
		if c.Response().Committed {
			return
		}

		// Return JSON for API endpoints
		if strings.HasPrefix(c.Request().URL.Path, "/api/") {
			c.JSON(code, map[string]interface{}{
				"error":   http.StatusText(code),
				"message": message,
				"path":    c.Request().URL.Path,
			})
			return
		}

		// Render appropriate template for HTML requests
		if code == http.StatusNotFound {
			if renderErr := tr.RenderWithStatus(c, "404.html", http.StatusNotFound, nil); renderErr != nil {
				Logger.Error("Failed to render 404 template", "error", renderErr)
				c.HTML(http.StatusNotFound, "<h1>404 - Not Found</h1>")
			}
			return
		}

		// For all other errors, show a simple error page with details
		c.HTML(code, fmt.Sprintf(
			"<h1>%d - %s</h1><p>%s</p><p><a href=\"/\">Back to home</a></p>",
			code, http.StatusText(code), message))
	}
}
