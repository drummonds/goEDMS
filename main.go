package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	config "github.com/drummonds/godocs/config"
	database "github.com/drummonds/godocs/database"
	engine "github.com/drummonds/godocs/engine"
	"github.com/drummonds/godocs/internal/build"
	"github.com/drummonds/godocs/internal/docs"
)

//go:embed web/app.wasm web/wasm_exec.js
var webFS embed.FS

//go:embed webapp/webapp.css webapp/wordcloud.css
var webappFS embed.FS

//go:embed public/built/favicon.ico public/built/404.html
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
	injectGlobals(logger) //inject the logger into all of the packages

	// Log version information
	version := build.GetVersion()
	logger.Info("Starting godocs", "version", version)
	fmt.Printf("\n🚀  godocs version %s\n", version)

	// Show info banner if using ephemeral database
	if serverConfig.DatabaseType == "ephemeral" {
		fmt.Println("\n" + strings.Repeat("=", 50))
		fmt.Println("🚀  EPHEMERAL DATABASE MODE")
		fmt.Println(strings.Repeat("=", 50))
		fmt.Println("• Database will be destroyed on exit")
		fmt.Println("• Perfect for testing and development")
		fmt.Println("• No persistent data storage")
		fmt.Println(strings.Repeat("=", 50) + "\n")
	}

	// Setup database (handles ephemeral, postgres, cockroachdb, sqlite)
	Logger.Info("Setting up database", "type", serverConfig.DatabaseType)
	db := database.NewRepository(serverConfig)
	defer db.Close()

	Logger.Info("Database setup complete")
	database.WriteConfigToDB(serverConfig, db) //writing the config to the database
	Logger.Info("Config written to DB")

	e := echo.New()
	Logger.Info("Echo created")

	// Custom 404 handler
	e.HTTPErrorHandler = createHTTPErrorHandler(e, publicFS, Logger)

	serverHandler := engine.ServerHandler{DB: db, Echo: e, ServerConfig: serverConfig} //injecting the database into the handler for routes
	docsHandler := docs.DocsHandler{DocsFS: docsFS, SwaggerUIFS: swaggerUIFS}          //handler for API documentation
	Logger.Info("About to initialize schedules")
	serverHandler.InitializeSchedules(db) //initialize all the cron jobs
	Logger.Info("Schedules initialized, about to run startup checks")
	serverHandler.StartupChecks() //Run all the sanity checks
	Logger.Info("Startup checks complete")

	// CORS configuration - allow frontend and backend ports
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{
			fmt.Sprintf("http://%s:%s", serverConfig.FrontendAddr, serverConfig.FrontendPort),
			fmt.Sprintf("http://localhost:%s", serverConfig.FrontendPort),
			fmt.Sprintf("http://127.0.0.1:%s", serverConfig.FrontendPort),
			fmt.Sprintf("http://localhost:%s", serverConfig.ListenAddrPort),
			fmt.Sprintf("http://127.0.0.1:%s", serverConfig.ListenAddrPort),
		},
		AllowMethods:     []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete, http.MethodPatch},
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowCredentials: true,
	}))

	// Request logging middleware - logs at debug level
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()

			start := time.Now()
			err := next(c)
			latency := time.Since(start)

			// Log API 404 errors at warning level, others at debug level
			if res.Status == http.StatusNotFound && strings.HasPrefix(req.URL.Path, "/api/") {
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

	Logger.Info("Setting up static file serving for frontend")

	// Serve wasm_exec.js from embedded filesystem (needed by frontend server)
	e.GET("/wasm_exec.js", func(c echo.Context) error {
		data, err := webFS.ReadFile("web/wasm_exec.js")
		if err != nil {
			return c.String(http.StatusNotFound, "wasm_exec.js not found")
		}
		return c.Blob(http.StatusOK, "application/javascript", data)
	})

	// Serve static assets from embedded filesystem
	webSubFS, _ := fs.Sub(webFS, "web")
	e.GET("/web/*", echo.WrapHandler(http.StripPrefix("/web/", http.FileServer(http.FS(webSubFS)))))

	// Serve CSS files from embedded filesystem
	e.GET("/webapp/webapp.css", func(c echo.Context) error {
		data, err := webappFS.ReadFile("webapp/webapp.css")
		if err != nil {
			return c.String(http.StatusNotFound, "webapp.css not found")
		}
		return c.Blob(http.StatusOK, "text/css", data)
	})

	e.GET("/webapp/wordcloud.css", func(c echo.Context) error {
		data, err := webappFS.ReadFile("webapp/wordcloud.css")
		if err != nil {
			return c.String(http.StatusNotFound, "wordcloud.css not found")
		}
		return c.Blob(http.StatusOK, "text/css", data)
	})

	// Serve favicon from embedded filesystem
	e.GET("/favicon.ico", func(c echo.Context) error {
		data, err := publicFS.ReadFile("public/built/favicon.ico")
		if err != nil {
			return c.String(http.StatusNotFound, "favicon.ico not found")
		}
		return c.Blob(http.StatusOK, "image/x-icon", data)
	})

	// Serve OpenAPI/Swagger documentation
	e.GET("/api/docs/swagger.json", docsHandler.GetSwaggerJSON)
	e.GET("/api/docs/openapi.yaml", docsHandler.GetOpenAPIYAML)
	e.GET("/api/docs/swagger-ui.css", docsHandler.GetSwaggerUICSS)
	e.GET("/api/docs/swagger-ui-bundle.js", docsHandler.GetSwaggerUIBundle)
	e.GET("/api/docs/swagger-ui-standalone-preset.js", docsHandler.GetSwaggerUIPreset)
	e.GET("/api/docs", docsHandler.GetSwaggerUI)

	// Inject backend API URL into the page
	e.GET("/config.js", func(c echo.Context) error {
		// Use the backend port (8000) as the API URL
		apiURL := fmt.Sprintf("http://localhost:%s", serverConfig.ListenAddrPort)
		configJS := fmt.Sprintf(`
// godocs Frontend Configuration
window.godocsConfig = {
    apiURL: "%s",
    newDocumentCount: %d
};
console.log("godocs Config loaded:", window.godocsConfig);
`, apiURL, serverConfig.NewDocumentNumber)
		c.Response().Header().Set("Content-Type", "application/javascript")
		return c.String(http.StatusOK, configJS)
	})

	Logger.Info("Logger enabled!!")

	//injecting database into the context so we can access it
	//Start the API routes - all under /api/* prefix for clarity

	// Document API routes
	e.GET("/api/documents/latest", serverHandler.GetLatestDocuments)
	e.GET("/api/documents/filesystem", serverHandler.GetDocumentFileSystem)
	e.GET("/api/document/:id", serverHandler.GetDocument)
	e.GET("/api/document/:id/thumbnail", serverHandler.GetDocumentThumbnail)
	e.GET("/api/document/:id/status", serverHandler.GetDocumentStatus)
	e.GET("/api/document/:id/text", serverHandler.GetDocumentText)
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

	// Document view routes are now handled dynamically by /document/view/:ulid route above

	// Serve a simple index page that redirects to the frontend
	e.GET("/", createIndexHandler(serverConfig))

	// For any other non-API routes, redirect to frontend
	e.Any("/*", createCatchAllHandler(serverConfig))

	// Create frontend server
	frontendServer := createFrontendServer(Logger, serverConfig)

	// Determine display addresses for console output
	backendDisplayAddr := serverConfig.ListenAddrIP
	if backendDisplayAddr == "" {
		backendDisplayAddr = "localhost"
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("🚀  Starting godocs Dual-Port Architecture")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("• Backend  (API + Static): http://%s:%s\n", backendDisplayAddr, serverConfig.ListenAddrPort)
	fmt.Printf("• Frontend (HTML Shell):   http://%s:%s\n", serverConfig.FrontendAddr, serverConfig.FrontendPort)
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("")
	fmt.Printf("✨  Access the app at http://%s:%s\n", serverConfig.FrontendAddr, serverConfig.FrontendPort)
	fmt.Printf("   (Clear separation: frontend routes on %s)\n", serverConfig.FrontendPort)
	fmt.Println(strings.Repeat("=", 50) + "\n")

	// Start backend server on port 8000 in a goroutine
	backendAddr := fmt.Sprintf("%s:%s", serverConfig.ListenAddrIP, serverConfig.ListenAddrPort)
	Logger.Info("Starting backend server", "address", backendAddr)

	go func() {
		if err := e.Start(backendAddr); err != nil && err != http.ErrServerClosed {
			Logger.Error("Backend server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Give backend a moment to start
	time.Sleep(500 * time.Millisecond)

	// Start frontend server on configured port in main goroutine
	frontendAddr := fmt.Sprintf("%s:%s", serverConfig.ListenAddrIP, serverConfig.FrontendPort)
	Logger.Info("Starting frontend server", "address", frontendAddr)

	if err := frontendServer.Start(frontendAddr); err != nil && err != http.ErrServerClosed {
		Logger.Error("Frontend server failed", "error", err)
		os.Exit(1)
	}
}

// createFrontendServer creates a minimal frontend server that serves HTML shell
func createFrontendServer(logger *slog.Logger, serverConfig config.ServerConfig) *echo.Echo {
	e := echo.New()
	e.HideBanner = true

	// CORS - allow requests from anywhere
	e.Use(middleware.CORS())

	// Request logging
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			latency := time.Since(start)

			logger.Debug("Frontend request",
				"method", c.Request().Method,
				"path", c.Request().URL.Path,
				"status", c.Response().Status,
				"latency", latency.String(),
			)
			return err
		}
	})

	// Get HTML template from separate file
	htmlTemplate := GetFrontendHTMLTemplate(serverConfig)

	// Build backend URL for redirects
	backendHost := serverConfig.ListenAddrIP
	if backendHost == "" {
		backendHost = "localhost"
	}
	backendURL := fmt.Sprintf("http://%s:%s", backendHost, serverConfig.ListenAddrPort)

	// Redirect backend-only routes directly to backend
	// /document/view/:ulid serves document files - must go to backend
	e.Any("/document/view/*", func(c echo.Context) error {
		logger.Info("Redirecting document view to backend",
			"path", c.Request().URL.Path,
			"backend", backendURL)
		return c.Redirect(http.StatusTemporaryRedirect, backendURL+c.Request().URL.Path)
	})

	// Redirect API calls to backend (in case browser accesses frontend URL for API)
	e.Any("/api/*", func(c echo.Context) error {
		logger.Info("Redirecting API call to backend",
			"path", c.Request().URL.Path,
			"backend", backendURL)
		return c.Redirect(http.StatusTemporaryRedirect, backendURL+c.Request().URL.Path)
	})

	// Serve the HTML shell for frontend routes
	// This allows any frontend route (/browse, /search, etc.) to work
	e.Any("/*", func(c echo.Context) error {
		return c.HTML(http.StatusOK, htmlTemplate)
	})

	return e
}

// isAddressInUse checks if the error is due to address already in use
func isAddressInUse(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "address already in use")
}
