//go:build js && wasm

package main

import (
	"fmt"
	"log/slog"
	"os"
	"syscall/js"

	"codeberg.org/hum3/godocs/config"
	"codeberg.org/hum3/godocs/database"
	"codeberg.org/hum3/godocs/internal/build"
	"github.com/labstack/echo/v4"
)

// Logger is global since we will need it everywhere (WASM version)
var Logger *slog.Logger

func main() {
	Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	database.Logger = Logger
	config.Logger = Logger

	version := build.GetVersion() + "-demo"
	fmt.Println("godocs WASM demo", version)

	// Init in-memory database
	db := database.NewMemDB()

	// Seed demo data
	seedDemoData(db)

	// Set up Echo router
	e := echo.New()
	e.HideBanner = true

	cfg := config.ServerConfig{
		DatabaseType: "in-memory (demo)",
	}

	tr := &TemplateRenderer{
		templateSet:       NewTemplateSet(),
		db:                db,
		config:            cfg,
		version:           version,
		isActionRunning:   func() bool { return false },
		checkThumbnail:    checkThumbnailEmbed,
		writeTagAliases:   func(string, database.Repository) error { return nil },
		runCleanupAsync:   func() (*database.Job, error) { return nil, fmt.Errorf("not available in demo") },
		runIngestionAsync: func() (*database.Job, error) { return nil, fmt.Errorf("not available in demo") },
	}

	// Register routes (SSR pages + read-only API subset)
	registerWASMRoutes(e, tr)

	// Custom error handler
	e.HTTPErrorHandler = createWASMErrorHandler(tr)

	// Export handleRequest to JS
	js.Global().Set("handleRequest", js.FuncOf(handleHTTPRequest(e)))

	// Signal ready
	if cb := js.Global().Get("wasmReady"); !cb.IsUndefined() {
		cb.Invoke()
	}
	fmt.Println("WASM ready, waiting for requests...")

	// Block forever
	select {}
}

// checkThumbnailEmbed always returns true for demo (thumbnails are pre-generated)
func checkThumbnailEmbed(docPath string) bool {
	return true
}
