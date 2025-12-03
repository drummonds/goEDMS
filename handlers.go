package main

import (
	"embed"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	config "github.com/drummonds/godocs/config"
)

// createHTTPErrorHandler returns a custom HTTP error handler that serves
// appropriate responses for 404 errors (JSON for API, HTML for other requests)
func createHTTPErrorHandler(e *echo.Echo, publicFS embed.FS, logger *slog.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		code := http.StatusInternalServerError
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
		}

		// For 404 errors, serve custom HTML page
		if code == http.StatusNotFound {
			// Check if this is an API request
			if strings.HasPrefix(c.Request().URL.Path, "/api/") {
				// Log 404 errors at warning level
				logger.Warn("API endpoint not found",
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
}

// createIndexHandler returns a handler that serves the backend index page
// with a redirect to the frontend
func createIndexHandler(serverConfig config.ServerConfig) echo.HandlerFunc {
	frontendURL := fmt.Sprintf("http://%s:%s/", serverConfig.FrontendAddr, serverConfig.FrontendPort)

	return func(c echo.Context) error {
		indexHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>godocs Backend</title>
    <meta http-equiv="refresh" content="0; url=%s">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
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
        <p>This is the backend API server on port %s</p>
        <p>Redirecting to frontend...</p>
        <a href="%s">Go to godocs Frontend →</a>
    </div>
</body>
</html>`, frontendURL, serverConfig.ListenAddrPort, frontendURL)
		return c.HTML(http.StatusOK, indexHTML)
	}
}

// createCatchAllHandler returns a handler that redirects non-API requests
// to the frontend server
func createCatchAllHandler(serverConfig config.ServerConfig) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Don't redirect API calls or document views
		if strings.HasPrefix(c.Request().URL.Path, "/api/") ||
			strings.HasPrefix(c.Request().URL.Path, "/document/view/") {
			return echo.NewHTTPError(http.StatusNotFound, "Not found")
		}
		// Redirect to frontend
		return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("http://%s:%s%s", serverConfig.FrontendAddr, serverConfig.FrontendPort, c.Request().URL.Path))
	}
}
