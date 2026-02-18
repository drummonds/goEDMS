package config

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"
)

// Logger is global since we will need it everywhere
var Logger *slog.Logger

// ServerConfig contains all of the server settings
type ServerConfig struct {
	ID                   int
	ListenAddrIP         string
	ListenAddrPort       string
	DatabaseType         string
	DatabaseHost         string
	DatabasePort         string
	DatabaseUser         string
	DatabasePassword     string
	DatabaseDbname       string
	DatabaseSslmode      string
	IngressPath          string
	IngressDelete        bool
	IngressMoveFolder    string
	IngressPreserve      bool
	UseSidecarTxt        bool
	DocumentPath         string
	ConfigPath           string // sibling of DocumentPath for persistent config (tag_aliases.json etc.)
	NewDocumentFolder    string //absolute path to new document folder
	NewDocumentFolderRel string //relative path to new document folder
	WebUIPass            bool
	ClientUsername       string
	ClientPassword       string
	PushBulletToken      string `json:"-"`
	TesseractPath        string
	UseReverseProxy      bool
	BaseURL              string
	IngressInterval      int
	FrontEndConfig
}

// FrontEndConfig stores frontend-related settings (persisted in database)
type FrontEndConfig struct {
	NewDocumentNumber int
	ServerAPIURL      string
}

// LoadEnvFile loads environment variables from a file (silently ignores if not found)
func LoadEnvFile(filename string) error {
	return godotenv.Load(filename)
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool gets a boolean environment variable with a default value
func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	boolVal, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return boolVal
}

// getEnvInt gets an integer environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intVal, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intVal
}

// SetupServer loads configuration and returns ServerConfig and Logger
func SetupServer() (ServerConfig, *slog.Logger) {
	serverConfigLive := ServerConfig{}
	frontEndConfigLive := FrontEndConfig{}

	// Load .env file (silently ignore if doesn't exist)
	// Try production location first, then local development files
	_ = godotenv.Load("/etc/godocs.env")
	_ = godotenv.Load(".env")
	_ = godotenv.Load("config.env")

	logger := setupLogging()
	Logger = logger

	// Load configuration from environment variables with defaults

	// Server configuration
	serverConfigLive.ListenAddrPort = getEnv("SERVER_PORT", "8000")
	serverConfigLive.ListenAddrIP = getEnv("SERVER_ADDR", "")

	// Database configuration
	serverConfigLive.DatabaseType = getEnv("DATABASE_TYPE", "postgres")
	serverConfigLive.DatabaseHost = getEnv("DATABASE_HOST", "localhost")
	serverConfigLive.DatabasePort = getEnv("DATABASE_PORT", "5432")
	serverConfigLive.DatabaseUser = getEnv("DATABASE_USER", "godocs")
	serverConfigLive.DatabasePassword = getEnv("DATABASE_PASSWORD", "")
	serverConfigLive.DatabaseDbname = getEnv("DATABASE_NAME", "godocs")
	serverConfigLive.DatabaseSslmode = getEnv("DATABASE_SSLMODE", "")

	logger.Info("Database configuration loaded", "type", serverConfigLive.DatabaseType)

	// Ingress configuration
	ingressDir := filepath.ToSlash(getEnv("INGRESS_PATH", "ingress"))
	ingressDirAbs, err := filepath.Abs(ingressDir)
	if err != nil {
		logger.Error("Failed creating absolute path for ingress directory", "error", err)
	}
	serverConfigLive.IngressPath = ingressDirAbs

	serverConfigLive.IngressInterval = getEnvInt("INGRESS_INTERVAL", 10)
	serverConfigLive.IngressPreserve = getEnvBool("INGRESS_PRESERVE_STRUCTURE", true)
	serverConfigLive.IngressDelete = getEnvBool("INGRESS_DELETE", true)  // Changed default to true - delete source files after ingestion
	serverConfigLive.UseSidecarTxt = getEnvBool("USE_SIDECAR_TXT", true) // Use sidecar .txt files for search text if present

	// IngressMoveFolder is now deprecated - we delete files instead of moving them
	// Kept for backwards compatibility but not created by default
	ingressMoveFolder := filepath.ToSlash(getEnv("INGRESS_MOVE_FOLDER", ""))
	if ingressMoveFolder != "" {
		ingressMoveFolderABS, err := filepath.Abs(ingressMoveFolder)
		if err != nil {
			logger.Error("Failed creating absolute path for ingress move folder", "error", err)
		}
		serverConfigLive.IngressMoveFolder = ingressMoveFolderABS
		if !serverConfigLive.IngressDelete {
			os.MkdirAll(ingressMoveFolderABS, os.ModePerm)
		}
	} else {
		serverConfigLive.IngressMoveFolder = ""
	}

	fmt.Println("Ingress Interval: ", serverConfigLive.IngressInterval)
	fmt.Println("\n========================================")
	fmt.Println("   godocs - Document Management System")
	fmt.Println("========================================")
	fmt.Printf("Server will start on: %s:%s\n", serverConfigLive.ListenAddrIP, serverConfigLive.ListenAddrPort)
	if serverConfigLive.ListenAddrIP == "" {
		fmt.Println("(Listening on all network interfaces)")
	}
	fmt.Printf("Detailed logs: %s\n", getEnv("LOG_FILE", "godocs.log"))
	fmt.Println("Initializing...")

	// Document storage configuration
	documentPathRelative := filepath.ToSlash(getEnv("DOCUMENT_PATH", "documents"))
	documentPathAbs, err := filepath.Abs(documentPathRelative)
	if err != nil {
		logger.Error("Error creating document path", "path", documentPathRelative, "error", err)
	}
	serverConfigLive.DocumentPath = documentPathAbs
	serverConfigLive.ConfigPath = filepath.Join(filepath.Dir(documentPathAbs), "config")

	newDocumentPath := filepath.ToSlash(getEnv("NEW_DOCUMENT_FOLDER", "New"))
	serverConfigLive.NewDocumentFolderRel = newDocumentPath
	serverConfigLive.NewDocumentFolder = filepath.Join(documentPathAbs, newDocumentPath)

	// OCR configuration
	tesseractPathConfig := getEnv("TESSERACT_PATH", "/usr/bin/tesseract")
	logger.Info("Checking tesseract executable path...")
	if _, err := os.Stat(tesseractPathConfig); err == nil {
		logger.Debug("Tesseract executable found", "path", tesseractPathConfig)
		serverConfigLive.TesseractPath = tesseractPathConfig
		logger.Info("Tesseract found and validated, OCR enabled", "path", tesseractPathConfig)
	} else {
		logger.Warn("Tesseract executable not found, OCR will be disabled", "path", tesseractPathConfig, "error", err)
		serverConfigLive.TesseractPath = ""
	}

	// Authentication configuration
	serverConfigLive.WebUIPass = getEnvBool("WEB_UI_AUTH", false)
	serverConfigLive.ClientUsername = getEnv("WEB_UI_USER", "admin")
	serverConfigLive.ClientPassword = getEnv("WEB_UI_PASSWORD", "Password1")

	// Reverse proxy configuration
	serverConfigLive.UseReverseProxy = getEnvBool("PROXY_ENABLED", false)
	serverConfigLive.BaseURL = getEnv("BASE_URL", "https://godocs.domain.org")

	if serverConfigLive.UseReverseProxy {
		logger.Info("Using Reverse Proxy", "baseURL", serverConfigLive.BaseURL)
	} else {
		logger.Info("Using relative URLs for API calls (frontend will use same host it was served from)")
	}

	// Frontend configuration (persisted in database)
	frontEndConfigLive.NewDocumentNumber = getEnvInt("NEW_DOCUMENT_COUNT", 5)
	frontEndConfigLive.ServerAPIURL = getEnv("SERVER_API_URL", "")
	serverConfigLive.FrontEndConfig = frontEndConfigLive

	// Notifications
	serverConfigLive.PushBulletToken = getEnv("PUSHBULLET_TOKEN", "")

	logger.Info("About to setup database", "type", serverConfigLive.DatabaseType)

	return serverConfigLive, logger
}

// setupLogging configures the application logger
func setupLogging() *slog.Logger {
	logLevel := getEnv("LOG_LEVEL", "debug")
	var level slog.Level

	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelDebug
	}

	handlerOptions := &slog.HandlerOptions{Level: level}

	logOutput := getEnv("LOG_OUTPUT", "file")
	var logWriter io.Writer

	if logOutput == "stdout" {
		logWriter = os.Stdout
	} else {
		logPath, err := filepath.Abs(filepath.ToSlash(getEnv("LOG_FILE", "godocs.log")))
		if err != nil {
			fmt.Printf("Error creating log file path: %v\n", err)
			logWriter = os.Stdout
		} else {
			logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				fmt.Printf("Failed to open log file: %v\n", err)
				logWriter = os.Stdout
			} else {
				logWriter = logFile
				fmt.Println("Logging to file: ", logPath)
			}
		}
	}

	handler := slog.NewTextHandler(logWriter, handlerOptions)
	return slog.New(handler)
}

// GetPreferredOutboundIP gets preferred outbound IP of this machine
func GetPreferredOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}

// checkExecutables verifies that an executable exists at the given path
func checkExecutables(tesseractPath string, logger *slog.Logger) error {
	_, err := os.Stat(tesseractPath)
	if err != nil {
		logger.Error("Cannot find tesseract executable at location specified", "path", tesseractPath)
		return err
	}
	logger.Debug("Tesseract executable found", "path", tesseractPath)
	return nil
}
