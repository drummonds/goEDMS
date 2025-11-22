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
	"github.com/drummonds/godocs/webapp"
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
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		code := http.StatusInternalServerError
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
		}

		// For 404 errors, serve custom HTML page
		if code == http.StatusNotFound {
			// Check if this is an API request
			if strings.HasPrefix(c.Request().URL.Path, "/api/") {
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

			// For non-API requests, serve custom 404 HTML from embedded filesystem
			if data, err := publicFS.ReadFile("public/built/404.html"); err == nil {
				c.HTMLBlob(http.StatusNotFound, data)
				return
			}

			// Fallback: serve inline HTML if embedded file doesn't exist
			c.HTML(http.StatusNotFound, `<!DOCTYPE html>
<html>
<head><title>404 - Not Found</title></head>
<body style="font-family: sans-serif; text-align: center; padding: 50px;">
	<h1>404 - Page Not Found</h1>
	<p>The page you're looking for doesn't exist.</p>
	<a href="/" style="color: #3498db; text-decoration: none; font-size: 18px;">← Go to Home Page</a>
</body>
</html>`)
			return
		}

		// For other errors, use default handler
		e.DefaultHTTPErrorHandler(err, c)
	}

	serverHandler := engine.ServerHandler{DB: db, Echo: e, ServerConfig: serverConfig} //injecting the database into the handler for routes
	Logger.Info("About to initialize schedules")
	serverHandler.InitializeSchedules(db) //initialize all the cron jobs
	Logger.Info("Schedules initialized, about to run startup checks")
	serverHandler.StartupChecks() //Run all the sanity checks
	Logger.Info("Startup checks complete")
	e.Use(middleware.CORSWithConfig(middleware.DefaultCORSConfig))

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

	Logger.Info("Setting up go-app WASM UI")
	appHandler := webapp.Handler()

	// Serve wasm_exec.js from embedded filesystem (go-app expects it here)
	e.GET("/wasm_exec.js", func(c echo.Context) error {
		data, err := webFS.ReadFile("web/wasm_exec.js")
		if err != nil {
			return c.String(http.StatusNotFound, "wasm_exec.js not found")
		}
		return c.Blob(http.StatusOK, "application/javascript", data)
	})

	// Register go-app specific resources
	e.GET("/app.js", echo.WrapHandler(appHandler))
	e.GET("/app.css", echo.WrapHandler(appHandler))
	e.GET("/manifest.webmanifest", echo.WrapHandler(appHandler))

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
	e.GET("/api/docs/swagger.json", func(c echo.Context) error {
		data, err := docsFS.ReadFile("docs/swagger.json")
		if err != nil {
			return c.String(http.StatusNotFound, "swagger.json not found")
		}
		return c.Blob(http.StatusOK, "application/json", data)
	})
	e.GET("/api/docs/openapi.yaml", func(c echo.Context) error {
		data, err := docsFS.ReadFile("docs/openapi.yaml")
		if err != nil {
			return c.String(http.StatusNotFound, "openapi.yaml not found")
		}
		return c.Blob(http.StatusOK, "text/yaml", data)
	})

	// Serve embedded Swagger UI assets
	e.GET("/api/docs/swagger-ui.css", func(c echo.Context) error {
		data, err := swaggerUIFS.ReadFile("static/swagger-ui/swagger-ui.css")
		if err != nil {
			return c.String(http.StatusNotFound, "swagger-ui.css not found")
		}
		return c.Blob(http.StatusOK, "text/css", data)
	})
	e.GET("/api/docs/swagger-ui-bundle.js", func(c echo.Context) error {
		data, err := swaggerUIFS.ReadFile("static/swagger-ui/swagger-ui-bundle.js")
		if err != nil {
			return c.String(http.StatusNotFound, "swagger-ui-bundle.js not found")
		}
		return c.Blob(http.StatusOK, "application/javascript", data)
	})
	e.GET("/api/docs/swagger-ui-standalone-preset.js", func(c echo.Context) error {
		data, err := swaggerUIFS.ReadFile("static/swagger-ui/swagger-ui-standalone-preset.js")
		if err != nil {
			return c.String(http.StatusNotFound, "swagger-ui-standalone-preset.js not found")
		}
		return c.Blob(http.StatusOK, "application/javascript", data)
	})

	// Swagger UI HTML page (fully self-contained, no CDN dependencies)
	e.GET("/api/docs", func(c echo.Context) error {
		html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>godocs API Documentation</title>
    <link rel="stylesheet" type="text/css" href="/api/docs/swagger-ui.css">
    <style>
        html { box-sizing: border-box; overflow-y: scroll; }
        *, *:before, *:after { box-sizing: inherit; }
        body { margin: 0; background: #fafafa; }
        .topbar { display: none; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="/api/docs/swagger-ui-bundle.js"></script>
    <script src="/api/docs/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                url: "/api/docs/swagger.json",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                layout: "BaseLayout"
            });
        };
    </script>
</body>
</html>`
		return c.HTML(http.StatusOK, html)
	})

	// Inject backend API URL into the page
	e.GET("/config.js", func(c echo.Context) error {
		configJS := fmt.Sprintf(`
// godocs Frontend Configuration
window.godocs_config = {
    apiURL: "%s",
    newDocumentCount: %d
};
console.log("godocs Config loaded:", window.godocs_config);
`, serverConfig.ServerAPIURL, serverConfig.NewDocumentNumber)
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

	// Document view routes (serve actual files - not JSON, so not under /api/*)
	serverHandler.AddDocumentViewRoutes() //Add all existing documents to direct view links

	// Serve go-app handler for all other routes (must be last)
	// The WASM app handles its own client-side routing and 404s via NotFoundPage component
	e.Any("/*", echo.WrapHandler(appHandler))

	if serverConfig.ListenAddrIP == "" {
		Logger.Info("No Ip Addr set, binding on ALL addresses")
	}

	Logger.Info("Starting HTTP server")

	// Try to start server with automatic port increment if port is in use
	maxRetries := 5
	startPort := serverConfig.ListenAddrPort
	var startErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		addr := fmt.Sprintf("%s:%s", serverConfig.ListenAddrIP, serverConfig.ListenAddrPort)
		Logger.Info("Attempting to start server", "address", addr, "attempt", attempt+1)

		startErr = e.Start(addr)

		// Check if error is "address already in use"
		if startErr != nil && isAddressInUse(startErr) {
			Logger.Warn("Port already in use, trying next port",
				"port", serverConfig.ListenAddrPort,
				"attempt", attempt+1,
				"max_attempts", maxRetries)

			// Increment port for next attempt
			portNum := 0
			fmt.Sscanf(serverConfig.ListenAddrPort, "%d", &portNum)
			portNum++
			serverConfig.ListenAddrPort = fmt.Sprintf("%d", portNum)

			if attempt == maxRetries-1 {
				Logger.Error("Failed to find available port after maximum retries",
					"start_port", startPort,
					"end_port", serverConfig.ListenAddrPort,
					"max_retries", maxRetries)
				Logger.Error("Please reboot your computer to free up ports or manually stop conflicting processes")
				os.Exit(1)
			}
		} else if startErr != nil {
			// Some other error occurred
			Logger.Error("Failed to start server", "error", startErr)
			os.Exit(1)
		} else {
			// Server started successfully
			break
		}
	}

	// If we got here and startErr is nil, server started successfully
	if startErr == nil && serverConfig.ListenAddrPort != startPort {
		Logger.Warn("Server started on alternative port due to conflicts",
			"requested_port", startPort,
			"actual_port", serverConfig.ListenAddrPort)
	}
}

// isAddressInUse checks if the error is due to address already in use
func isAddressInUse(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "address already in use")
}
