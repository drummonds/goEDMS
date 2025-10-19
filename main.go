package main

import (
	"fmt"
	"log/slog"
	"os"

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
	serverConfig, logger := config.SetupServer()
	injectGlobals(logger) //inject the logger into all of the packages
	Logger.Info("About to setup database")
	db := database.SetupDatabase()
	Logger.Info("Database setup complete, about to setup search DB")
	searchDB, err := database.SetupSearchDB()
	if err != nil {
		Logger.Error("Unable to setup index database", "error", err)
		os.Exit(1)
	}
	Logger.Info("Search DB setup complete")
	defer db.Close()
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

	// Serve go-app UI at /app
	Logger.Info("Setting up go-app UI at /app")
	appHandler := webapp.Handler()

	// Serve wasm_exec.js at root FIRST (go-app expects it here)
	e.GET("/wasm_exec.js", func(c echo.Context) error {
		return c.File("web/wasm_exec.js")
	})

	// Register go-app specific resources
	e.GET("/app.js", echo.WrapHandler(appHandler))
	e.GET("/app.css", echo.WrapHandler(appHandler))
	e.GET("/manifest.webmanifest", echo.WrapHandler(appHandler))

	// Serve the go-app handler for all /app routes
	e.Any("/app*", echo.WrapHandler(appHandler))

	// Serve static assets
	e.Static("/web", "web")
	e.File("/webapp/webapp.css", "webapp/webapp.css")

	// Serve React Frontend at root (must be last to avoid catching go-app routes)
	e.Static("/", "public/built")
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

	if serverConfig.ListenAddrIP == "" {
		Logger.Info("No Ip Addr set, binding on ALL addresses")
	}

	Logger.Info("Starting HTTP server")

	if err := e.Start(fmt.Sprintf("%s:%s", serverConfig.ListenAddrIP, serverConfig.ListenAddrPort)); err != nil {
		Logger.Error("Failed to start server", "error", err)
		os.Exit(1)
	}

}
