package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	config "github.com/drummonds/godocs/config"
)

// Logger is global since we will need it everywhere
var Logger *slog.Logger

// HTML template that loads all resources from backend server
const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>godocs</title>
    <meta name="description" content="Electronic Document Management System">
    <link rel="icon" href="http://localhost:8000/favicon.ico">
    <link rel="stylesheet" href="http://localhost:8000/webapp/webapp.css">
    <link rel="stylesheet" href="http://localhost:8000/webapp/wordcloud.css">
    <script src="http://localhost:8000/wasm_exec.js"></script>
    <script src="http://localhost:8000/config.js"></script>
</head>
<body>
    <div id="app"></div>
    <script>
        // Load and run the WASM application
        const go = new Go();
        WebAssembly.instantiateStreaming(
            fetch("http://localhost:8000/web/app.wasm"),
            go.importObject
        ).then((result) => {
            go.run(result.instance);
        }).catch((err) => {
            console.error("Failed to load WASM:", err);
        });
    </script>
</body>
</html>`

func main() {
	// Parse command-line flags
	port := flag.String("port", "8001", "Port to run frontend server on")
	backendURL := flag.String("backend", "http://localhost:8000", "Backend server URL")
	flag.Parse()

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
		"port", *port,
		"backendURL", *backendURL)

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
	addr := fmt.Sprintf(":%s", *port)
	Logger.Info("Starting Frontend Server", "address", addr, "backend", *backendURL)
	fmt.Printf("\n✅  Frontend Server running on %s\n", addr)
	fmt.Printf("🎨  Open http://localhost:%s in your browser\n", *port)
	fmt.Printf("📡  Resources loaded from: %s\n", *backendURL)
	fmt.Printf("📡  API calls go directly to: %s/api/\n\n", *backendURL)

	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		Logger.Error("Server failed to start", "error", err)
	}
}
