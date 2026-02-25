package main

import (
	"flag"
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
)

// Logger is global since we will need it everywhere
var Logger *slog.Logger

// injectGlobals injects all of our globals into their packages
func injectGlobals(logger *slog.Logger) {
	Logger = logger
	database.Logger = Logger
	config.Logger = Logger
	engine.Logger = Logger
}

// @title godocs Backend API
// @version 1.0
// @description Electronic Document Management System API - Backend service for document storage, search, and management
// @description Supports document ingestion, full-text search, word cloud generation, and file system browsing

// @contact.name API Support
// @contact.url https://github.com/drummonds/godocs

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8000
// @BasePath /
// @schemes http https

// @tag.name Documents
// @tag.description Document management operations

// @tag.name Search
// @tag.description Full-text search and indexing operations

// @tag.name Folders
// @tag.description Folder management operations

// @tag.name Admin
// @tag.description Administrative operations (ingestion, cleanup)

// @tag.name WordCloud
// @tag.description Word frequency analysis and word cloud generation

// @tag.name Health
// @tag.description Service health check

func main() {
	// Parse command-line flags
	port := flag.String("port", "8000", "Port to run backend server on")
	flag.Parse()

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("godocs Backend API Server")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("API endpoints under /api/*")
	fmt.Println(strings.Repeat("=", 50) + "\n")

	serverConfig, logger := config.SetupServer()
	injectGlobals(logger) //inject the logger into all of the packages

	// Show info banner if using ephemeral database
	if serverConfig.DatabaseType == "ephemeral" {
		fmt.Println("🚀  EPHEMERAL DATABASE MODE")
		fmt.Println("• Database will be destroyed on exit")
		fmt.Println()
	}

	// Setup document repository
	repo := database.NewRepository(serverConfig)
	defer repo.Close()

	// Write config to database if it's a fresh ephemeral database
	if serverConfig.DatabaseType == "ephemeral" {
		database.WriteConfigToDB(serverConfig, repo)
	}

	// Initialize Echo
	e := echo.New()
	e.HideBanner = true

	// Custom 404 handler for API endpoints
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		code := http.StatusInternalServerError
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
		}

		if code == http.StatusNotFound {
			// Log 404 errors at warning level
			Logger.Warn("API endpoint not found",
				"method", c.Request().Method,
				"path", c.Request().URL.Path,
				"ip", c.RealIP(),
				"user_agent", c.Request().UserAgent())

			// Return JSON for API endpoints
			c.JSON(http.StatusNotFound, map[string]string{
				"error":   "Not Found",
				"message": "The requested API endpoint does not exist",
				"path":    c.Request().URL.Path,
			})
			return
		}

		// For other errors, use default handler
		e.DefaultHTTPErrorHandler(err, c)
	}

	serverHandler := engine.ServerHandler{DB: repo, Echo: e, ServerConfig: serverConfig}
	serverHandler.InitJobContext()
	Logger.Info("Initializing backend services...")
	serverHandler.InitializeSchedules(repo) //initialize all the cron jobs
	serverHandler.StartupChecks()           //Run all the sanity checks
	Logger.Info("Backend services initialized")

	// Request logging middleware - logs 404s at warning level, others at debug level
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()

			start := time.Now()
			err := next(c)
			latency := time.Since(start)

			// Log API 404 errors at warning level, others at debug level
			if res.Status == http.StatusNotFound {
				Logger.Warn("API endpoint not found",
					"method", req.Method,
					"path", req.URL.Path,
					"status", res.Status,
					"latency", latency.String(),
					"ip", c.RealIP(),
					"user_agent", req.UserAgent(),
				)
			} else {
				Logger.Debug("HTTP request",
					"method", req.Method,
					"path", req.URL.Path,
					"status", res.Status,
					"latency", latency.String(),
					"ip", c.RealIP(),
					"user_agent", req.UserAgent(),
				)
			}

			return err
		}
	})

	Logger.Info("Setting up API routes...")

	// Document API routes
	e.GET("/api/documents/latest", serverHandler.GetLatestDocuments)
	e.GET("/api/documents/filesystem", serverHandler.GetDocumentFileSystem)
	e.GET("/api/document/:id", serverHandler.GetDocument)
	e.GET("/api/document/:id/thumbnail", serverHandler.GetDocumentThumbnail)
	e.DELETE("/api/document/*", serverHandler.DeleteFile)
	e.PATCH("/api/document/move/*", serverHandler.MoveDocuments)
	e.POST("/api/document/upload", serverHandler.UploadDocuments)

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
	e.GET("/api/jobs/:id", serverHandler.GetJob)

	// Document view routes (serve actual PDF/document files)
	// These are not under /api/* because they serve files, not JSON
	serverHandler.AddDocumentViewRoutes()

	// Health check endpoint
	e.GET("/api/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status":  "healthy",
			"service": "godocs Backend API",
		})
	})

	// Static file routes
	e.File("/favicon.ico", "public/built/favicon.ico")

	// Override port if specified via flag
	if *port != "8000" {
		serverConfig.ListenAddrPort = *port
	}

	// Start server
	addr := fmt.Sprintf("%s:%s", serverConfig.ListenAddrIP, serverConfig.ListenAddrPort)
	Logger.Info("Starting Backend Server", "address", addr)
	fmt.Printf("\nBackend Server running on %s\n", addr)
	fmt.Printf("API endpoints: http://%s/api/\n", addr)
	fmt.Printf("Health check: http://%s/api/health\n\n", addr)

	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		Logger.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}
