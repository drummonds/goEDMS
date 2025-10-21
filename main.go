package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	config "github.com/drummonds/goEDMS/config"
	database "github.com/drummonds/goEDMS/database"
	engine "github.com/drummonds/goEDMS/engine"
	"github.com/drummonds/goEDMS/webapp"
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

func main() {
	// Parse command-line flags
	devMode := flag.Bool("dev", false, "Run in development mode with ephemeral PostgreSQL")
	flag.Parse()

	serverConfig, logger := config.SetupServer()
	injectGlobals(logger) //inject the logger into all of the packages

	// Setup database based on dev mode or configuration
	var db database.DBInterface
	if *devMode {
		fmt.Println("\n" + strings.Repeat("=", 50))
		fmt.Println("🚀  DEVELOPMENT MODE - Ephemeral PostgreSQL")
		fmt.Println(strings.Repeat("=", 50))
		fmt.Println("• Database will be destroyed on exit")
		fmt.Println("• Perfect for testing and development")
		fmt.Println("• No persistent data storage")
		fmt.Println(strings.Repeat("=", 50) + "\n")

		Logger.Info("Starting ephemeral PostgreSQL for development")
		ephemeralDB, err := database.SetupEphemeralPostgresDatabase()
		if err != nil {
			Logger.Error("Failed to setup ephemeral PostgreSQL", "error", err)
			os.Exit(1)
		}
		db = ephemeralDB
		// Ensure cleanup happens on exit
		defer func() {
			Logger.Info("Shutting down ephemeral PostgreSQL...")
			ephemeralDB.Close()
		}()
	} else {
		Logger.Info("About to setup database", "type", serverConfig.DatabaseType)
		db = database.SetupDatabase(serverConfig.DatabaseType, serverConfig.DatabaseConnString)
		defer db.Close()
	}
	Logger.Info("Database setup complete, about to setup search DB")
	searchDB, err := database.SetupSearchDB()
	if err != nil {
		Logger.Error("Unable to setup index database", "error", err)
		os.Exit(1)
	}
	Logger.Info("Search DB setup complete")
	defer searchDB.Close()
	database.WriteConfigToDB(serverConfig, db) //writing the config to the database
	Logger.Info("Config written to DB")

	e := echo.New()
	Logger.Info("Echo created")
	serverHandler := engine.ServerHandler{DB: db, SearchDB: searchDB, Echo: e, ServerConfig: serverConfig} //injecting the database into the handler for routes
	Logger.Info("About to initialize schedules")
	serverHandler.InitializeSchedules(db, searchDB) //initialize all the cron jobs
	Logger.Info("Schedules initialized, about to run startup checks")
	serverHandler.StartupChecks() //Run all the sanity checks
	Logger.Info("Startup checks complete")
	e.Use(middleware.CORSWithConfig(middleware.DefaultCORSConfig))

	Logger.Info("Setting up go-app WASM UI")
	appHandler := webapp.Handler()

	// Serve wasm_exec.js (go-app expects it here)
	e.GET("/wasm_exec.js", func(c echo.Context) error {
		return c.File("web/wasm_exec.js")
	})

	// Register go-app specific resources
	e.GET("/app.js", echo.WrapHandler(appHandler))
	e.GET("/app.css", echo.WrapHandler(appHandler))
	e.GET("/manifest.webmanifest", echo.WrapHandler(appHandler))

	// Serve static assets
	e.Static("/web", "web")
	e.File("/webapp/webapp.css", "webapp/webapp.css")
	e.File("/favicon.ico", "public/built/favicon.ico")

	Logger.Info("Logger enabled!!")

	//injecting database into the context so we can access it
	//Start the API routes
	e.GET("/home", serverHandler.GetLatestDocuments)
	e.GET("/documents/filesystem", serverHandler.GetDocumentFileSystem)
	e.GET("/document/:id", serverHandler.GetDocument)
	e.DELETE("/document/*", serverHandler.DeleteFile)
	e.PATCH("document/move/*", serverHandler.MoveDocuments)
	e.POST("/document/upload", serverHandler.UploadDocuments)
	serverHandler.AddDocumentViewRoutes() //Add all existing documents to direct view links
	e.GET("/folder/:folder", serverHandler.GetFolder)
	e.POST("/folder/*", serverHandler.CreateFolder)
	e.GET("/search/*", serverHandler.SearchDocuments)

	// Admin API routes
	e.POST("/api/ingest", serverHandler.RunIngestNow)
	e.POST("/api/clean", serverHandler.CleanDatabase)
	e.GET("/api/about", serverHandler.GetAboutInfo)

	// Serve go-app handler for all other routes (must be last)
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
