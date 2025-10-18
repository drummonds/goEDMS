package config

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Logger is global since we will need it everywhere
var Logger *slog.Logger

// ServerConfig contains all of the server settings defined in the TOML file
type ServerConfig struct {
	StormID              int `storm:"id"`
	ListenAddrIP         string
	ListenAddrPort       string
	IngressPath          string
	IngressDelete        bool
	IngressMoveFolder    string
	IngressPreserve      bool
	DocumentPath         string
	NewDocumentFolder    string //absolute path to new document folder
	NewDocumentFolderRel string //relative path to new document folder Needed for multiple levels deep.
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

// FrontEndConfig stores all of the frontend settings
type FrontEndConfig struct {
	NewDocumentNumber int
	ServerAPIURL      string
}

func defaultConfig() ServerConfig { //TODO: Do I even bother, if config fails most likely not worth continuing
	var ServerConfigDefault ServerConfig
	//Config.AppVersion
	//zerolog.SetGlobalLevel(zerolog.WarnLevel)
	ServerConfigDefault.DocumentPath = "documents"
	ServerConfigDefault.IngressPath = "ingress"
	ServerConfigDefault.WebUIPass = false
	ServerConfigDefault.UseReverseProxy = false
	return ServerConfigDefault
}

// SetupServer does the initial configuration
func SetupServer() (ServerConfig, *slog.Logger) {
	var serverConfigLive ServerConfig
	viper.AddConfigPath("config/")
	viper.AddConfigPath(".")
	viper.SetConfigName("serverConfig")
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %s \n", err))
	}
	logger := setupLogging()
	ingressDir := filepath.ToSlash(viper.GetString("ingress.IngressPath")) //Converting the string literal into a filepath
	ingressDirAbs, err := filepath.Abs(ingressDir)                         //Converting to an absolute file path
	if err != nil {
		logger.Error("Failed creating absolute path for ingress directory", "error", err)
	}
	serverConfigLive.IngressPath = ingressDirAbs
	logger.Info("Base Logger is setup!")
	serverConfigLive.ListenAddrPort = viper.GetString("serverConfig.ServerPort")
	serverConfigLive.ListenAddrIP = viper.GetString("serverConfig.ServerAddr")
	serverConfigLive.IngressInterval = viper.GetInt("ingress.scheduling.IngressInterval")
	serverConfigLive.IngressPreserve = viper.GetBool("ingress.handling.PreserveDirStructure")
	serverConfigLive.IngressDelete = viper.GetBool("ingress.completed.IngressDeleteOnProcess")
	ingressMoveFolder := filepath.ToSlash(viper.GetString("ingress.completed.IngressMoveFolder"))
	ingressMoveFolderABS, err := filepath.Abs(ingressMoveFolder)
	if err != nil {
		logger.Error("Failed creating absolute path for ingress move folder", "error", err)
	}
	serverConfigLive.IngressMoveFolder = ingressMoveFolderABS
	os.MkdirAll(ingressMoveFolderABS, os.ModePerm) //creating the directory for moving now
	fmt.Println("Ingress Interval: ", serverConfigLive.IngressInterval)
	documentPathRelative := filepath.ToSlash(viper.GetString("documentLibrary.DocumentFileSystemLocation"))
	serverConfigLive.DocumentPath, err = filepath.Abs(documentPathRelative)
	if err != nil {
		logger.Error("Failed creating absolute path for document library", "error", err)
	}
	newDocumentPath := filepath.ToSlash(viper.GetString("documentLibrary.DefaultNewDocumentFolder"))
	serverConfigLive.NewDocumentFolderRel = newDocumentPath
	serverConfigLive.NewDocumentFolder = filepath.Join(serverConfigLive.DocumentPath, newDocumentPath)
	tesseractPathConfig := viper.GetString("ocr.TesseractBin")
	if tesseractPathConfig != "" {
		serverConfigLive.TesseractPath, err = filepath.Abs(filepath.ToSlash(tesseractPathConfig))
		if err != nil {
			logger.Warn("Failed creating absolute path for tesseract binary, OCR will be disabled", "error", err)
			serverConfigLive.TesseractPath = ""
		} else {
			logger.Info("Checking tesseract executable path...")
			err = checkExecutables(serverConfigLive.TesseractPath, logger)
			if err != nil {
				logger.Warn("Tesseract executable not found, OCR will be disabled", "path", serverConfigLive.TesseractPath)
				serverConfigLive.TesseractPath = ""
			} else {
				logger.Info("Tesseract found and validated, OCR enabled", "path", serverConfigLive.TesseractPath)
			}
		}
	} else {
		logger.Info("No Tesseract path configured, OCR will be disabled")
	}
	serverConfigLive.UseReverseProxy = viper.GetBool("reverseProxy.ProxyEnabled")
	serverConfigLive.BaseURL = viper.GetString("reverseProxy.BaseURL")
	os.MkdirAll(serverConfigLive.NewDocumentFolder, os.ModePerm)
	frontEndConfigLive := setupFrontEnd(serverConfigLive, logger)
	serverConfigLive.FrontEndConfig = frontEndConfigLive
	return serverConfigLive, logger
}

func setupFrontEnd(serverConfigLive ServerConfig, logger *slog.Logger) FrontEndConfig {
	var frontEndConfigLive FrontEndConfig
	var frontEndURL string
	frontEndConfigLive.NewDocumentNumber = viper.GetInt("frontend.NewDocumentNumber") //number of new documents to display //TODO: maybe not using this...
	frontEndConfigLive.ServerAPIURL = viper.GetString("serverConfig.APIURL")          //Used for docker to manually specify URL for backend
	//TODO add check for docker container api URL and generate it here
	if frontEndConfigLive.ServerAPIURL != "" { //if this is NOT blank it is docker specifying the URL
		frontEndURL = fmt.Sprintf("http://%s", frontEndConfigLive.ServerAPIURL)
	} else {
		if serverConfigLive.UseReverseProxy { //if using a proxy set the proxy URL
			frontEndURL = serverConfigLive.BaseURL
		} else { //If NOT using a proxy determine the IP URL
			if serverConfigLive.ListenAddrIP == "" { //If no IP listed attempt to discover the default IP addr
				ipAddr, err := getDefaultIP(logger)
				if err != nil {
					logger.Error("WARNING! Unable to determine default IP, frontend-config.js may need to be manually modified for goEDMS to work!", "error", err)
					frontEndURL = fmt.Sprintf("http://%s:%s", serverConfigLive.ListenAddrIP, serverConfigLive.ListenAddrPort)
				} else {
					frontEndURL = fmt.Sprintf("http://%s:%s", *ipAddr, serverConfigLive.ListenAddrPort)
				}
			} else { //If IP addr listed then just use that in the IP URL
				frontEndURL = fmt.Sprintf("http://%s:%s", serverConfigLive.ListenAddrIP, serverConfigLive.ListenAddrPort)
			}

		}
	}
	var frontEndJS = fmt.Sprintf(`window['runConfig'] = {
		apiUrl: "%s"
	}`, frontEndURL) //Creating the react API file so the frontend will connect with the backend
	err := os.WriteFile("public/built/frontend-config.js", []byte(frontEndJS), 0644)
	if err != nil {
		logger.Error("Error writing frontend config to public/built/frontend-config.js", "error", err)
		os.Exit(1)
	}
	return frontEndConfigLive
}

func getDefaultIP(logger *slog.Logger) (*string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80") //attempting to determine the default IP by connecting out
	if err != nil {
		logger.Error("Error discovering Local IP! Either network connection error (outbound connection is used to determine default IP) or error determining default IP!", "error", err)
		return nil, err
	}
	defer conn.Close()
	localaddr := conn.LocalAddr().(*net.UDPAddr).IP.String()
	logger.Info("Local IP Determined", "ip", localaddr)
	return &localaddr, nil
}

func setupLogging() *slog.Logger {
	logLevelString := viper.GetString("logging.Level")
	var loglevel slog.Level
	switch logLevelString {
	case "Debug", "debug":
		loglevel = slog.LevelDebug
	case "Info", "info":
		loglevel = slog.LevelInfo
	case "Warn", "warn":
		loglevel = slog.LevelWarn
	case "Error", "error":
		loglevel = slog.LevelError
	default:
		loglevel = slog.LevelWarn
	}

	var logWriter io.Writer
	logOutput := viper.GetString("logging.OutputPath")
	if logOutput == "file" {
		logPath, err := filepath.Abs(filepath.ToSlash(viper.GetString("logging.LogFileLocation")))
		if err != nil {
			fmt.Println("Unable to create log file path: ", err)
			logPath = "output.log"
		}
		logFile, err := os.Create(logPath)
		if err != nil {
			fmt.Println("Unable to create log file: ", err)
			logWriter = os.Stdout
		} else {
			logWriter = logFile
			fmt.Println("Logging to file: ", logPath)
		}
	} else {
		logWriter = os.Stdout
		fmt.Println("Will be logging to stdout...")
	}

	opts := &slog.HandlerOptions{
		Level: loglevel,
	}
	handler := slog.NewTextHandler(logWriter, opts)
	logger := slog.New(handler)
	return logger
}

func checkExecutables(tesseractPath string, logger *slog.Logger) error {
	_, err := os.Stat(tesseractPath)
	if err != nil {
		logger.Error("Cannot find tesseract executable at location specified", "path", tesseractPath)
		return err
	}
	logger.Debug("Tesseract executable found", "path", tesseractPath)
	return nil
}
