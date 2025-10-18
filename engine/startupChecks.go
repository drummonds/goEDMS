package engine

import (
	"fmt"
	"os"

	"github.com/deranjer/goEDMS/config"
	"github.com/deranjer/goEDMS/database"
)

// StartupChecks performs all the checks to make sure everything works
func (serverHandler *ServerHandler) StartupChecks() error {
	serverConfig, err := database.FetchConfigFromDB(serverHandler.DB)
	if err != nil {
		Logger.Error("Error fetching config", "error", err)
		return err
	}
	tesseractChecks(serverConfig)
	return nil
}

func tesseractChecks(serverConfig config.ServerConfig) error {
	if serverConfig.TesseractPath == "" {
		Logger.Info("Tesseract not configured, OCR functionality will be unavailable")
		return nil
	}

	tesseractInfo, err := os.Stat(serverConfig.TesseractPath)
	if err != nil {
		Logger.Warn("Tesseract executable not found, OCR will be disabled", "path", serverConfig.TesseractPath, "error", err)
		return nil // Don't return error, just continue without OCR
	}
	if tesseractInfo.IsDir() {
		Logger.Warn("Tesseract path is a directory, not an executable, OCR will be disabled", "path", serverConfig.TesseractPath)
		return nil // Don't return error, just continue without OCR
	}
	fmt.Println("Tesseract Perms: ", tesseractInfo.Mode())
	if tesseractInfo.Mode() == 0111 {
		fmt.Println("Mode is executable?", tesseractInfo.Mode())
	}
	Logger.Info("Tesseract executable found and validated, OCR enabled", "path", serverConfig.TesseractPath)
	return nil
}
