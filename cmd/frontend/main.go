package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	config "github.com/drummonds/godocs/config"
)

// Logger is global since we will need it everywhere
var Logger *slog.Logger

// getHTMLTemplate returns the HTML template with the backend URL injected
func getHTMLTemplate(backendURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>godocs</title>
    <meta name="description" content="Electronic Document Management System">
    <link rel="icon" href="%s/favicon.ico">
    <link rel="stylesheet" href="%s/webapp/webapp.css">
    <link rel="stylesheet" href="%s/webapp/wordcloud.css">
    <script src="%s/wasm_exec.js"></script>
    <script>
        // Set backend URL for WASM app
        window.godocsBackendURL = '%s';
        window.godocsConfig = {
            apiURL: '%s'
        };
    </script>
</head>
<body>
    <div id="app"></div>
    <script>
        // Load and run the WASM application
        const go = new Go();
        WebAssembly.instantiateStreaming(
            fetch("%s/web/app.wasm"),
            go.importObject
        ).then((result) => {
            go.run(result.instance);
        }).catch((err) => {
            console.error("Failed to load WASM:", err);
        });
    </script>
</body>
</html>`, backendURL, backendURL, backendURL, backendURL, backendURL, backendURL, backendURL)
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	// Load frontend.env if it exists
	_ = config.LoadEnvFile("frontend.env")

	// Get configuration from environment variables
	port := getEnv("FRONTEND_PORT", "8001")
	backendURL := getEnv("SERVER_API_URL", "http://localhost:8000")

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("🎨  godocs Frontend Server")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("• Serves HTML shell for WASM app")
	fmt.Println("• All resources loaded from backend")
	fmt.Println("• Client makes direct API calls to backend")
	fmt.Println(strings.Repeat("=", 50) + "\n")

	_, logger := config.SetupFrontend()
	Logger = logger
	config.Logger = logger

	Logger.Info("Frontend server starting",
		"port", port,
		"backendURL", backendURL)

	// Generate HTML template with backend URL
	htmlTemplate := getHTMLTemplate(backendURL)

	// Initialize Echo
	e := echo.New()
	e.HideBanner = true

	// CORS - allow requests from anywhere
	e.Use(middleware.CORS())

	// Request logging
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "method=${method}, uri=${uri}, status=${status}\n",
	}))

	// Serve the HTML shell for ALL routes
	// This allows any frontend route (/browse, /search, etc.) to work
	e.Any("/*", func(c echo.Context) error {
		return c.HTML(http.StatusOK, htmlTemplate)
	})

	// Start server
	addr := fmt.Sprintf(":%s", port)
	Logger.Info("Starting Frontend Server", "address", addr, "backend", backendURL)
	fmt.Printf("\n✅  Frontend Server running on %s\n", addr)
	fmt.Printf("🎨  Open http://localhost:%s in your browser\n", port)
	fmt.Printf("📡  Resources loaded from: %s\n", backendURL)
	fmt.Printf("📡  API calls go directly to: %s/api/\n\n", backendURL)

	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		Logger.Error("Server failed to start", "error", err)
	}
}
