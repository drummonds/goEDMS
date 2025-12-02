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
	docsHandler := docs.DocsHandler{DocsFS: docsFS, SwaggerUIFS: swaggerUIFS}          //handler for API documentation
	Logger.Info("About to initialize schedules")
	serverHandler.InitializeSchedules(db) //initialize all the cron jobs
	Logger.Info("Schedules initialized, about to run startup checks")
	serverHandler.StartupChecks() //Run all the sanity checks
	Logger.Info("Startup checks complete")

	// CORS configuration - allow frontend from port 8001 and localhost variations
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"http://localhost:8001", "http://127.0.0.1:8001", "http://localhost:8000", "http://127.0.0.1:8000"},
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

	// Serve wasm_exec.js from embedded filesystem (needed by frontend on port 8001)
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
	// (Old approach: serverHandler.AddDocumentViewRoutes() registered individual routes at startup)

	// Serve a simple index page that redirects to the frontend on port 8001
	// Backend should not serve the full app - that's the frontend's job
	e.GET("/", func(c echo.Context) error {
		indexHTML := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>godocs Backend</title>
    <meta http-equiv="refresh" content="0; url=http://localhost:8001/">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
        }
        .container {
            text-align: center;
            padding: 2rem;
        }
        h1 { font-size: 2.5rem; margin-bottom: 1rem; }
        p { font-size: 1.2rem; margin-bottom: 2rem; }
        a {
            display: inline-block;
            padding: 1rem 2rem;
            background: white;
            color: #667eea;
            text-decoration: none;
            border-radius: 8px;
            font-weight: 600;
            transition: transform 0.2s;
        }
        a:hover { transform: scale(1.05); }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔧 godocs Backend</h1>
        <p>This is the backend API server on port 8000</p>
        <p>Redirecting to frontend...</p>
        <a href="http://localhost:8001/">Go to godocs Frontend →</a>
    </div>
</body>
</html>`
		return c.HTML(http.StatusOK, indexHTML)
	})

	// For any other non-API routes, redirect to frontend
	e.Any("/*", func(c echo.Context) error {
		// Don't redirect API calls or document views
		if strings.HasPrefix(c.Request().URL.Path, "/api/") ||
			strings.HasPrefix(c.Request().URL.Path, "/document/view/") {
			return echo.NewHTTPError(http.StatusNotFound, "Not found")
		}
		// Redirect to frontend
		return c.Redirect(http.StatusTemporaryRedirect, "http://localhost:8001"+c.Request().URL.Path)
	})

	// Create frontend server on port 8001
	frontendServer := createFrontendServer(Logger, serverConfig)

	if serverConfig.ListenAddrIP == "" {
		Logger.Info("No IP Addr set, binding on ALL addresses")
	}

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("🚀  Starting godocs Dual-Port Architecture")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("• Backend  (API + Static): http://localhost:%s\n", serverConfig.ListenAddrPort)
	fmt.Println("• Frontend (HTML Shell):   http://localhost:8001")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("")
	fmt.Println("✨  Access the app at http://localhost:8001")
	fmt.Println("   (Clear separation: frontend routes on 8001)")
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

	// Serve the HTML shell for ALL routes
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
