package engine

import (
	"fmt"
	"os"

	"codeberg.org/hum3/godocs/config"
	"codeberg.org/hum3/godocs/database"
)

// StartupChecks performs all the checks to make sure everything works
func (serverHandler *ServerHandler) StartupChecks() error {
	serverConfig, err := database.FetchConfigFromDB(serverHandler.DB)
	if err != nil {
		Logger.Error("Error fetching config", "error", err)
		return err
	}
	tesseractChecks(serverConfig)
	ingressDirectoryChecks(serverConfig)
	documentDirectoryChecks(serverConfig)
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

// ingressDirectoryChecks ensures the ingress directory exists
func ingressDirectoryChecks(serverConfig config.ServerConfig) error {
	if serverConfig.IngressPath == "" {
		Logger.Warn("Ingress path not configured")
		return nil
	}

	// Check if directory exists
	ingressInfo, err := os.Stat(serverConfig.IngressPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create the directory
			Logger.Info("Creating ingress directory", "path", serverConfig.IngressPath)
			err = os.MkdirAll(serverConfig.IngressPath, 0755)
			if err != nil {
				Logger.Error("Failed to create ingress directory", "path", serverConfig.IngressPath, "error", err)
				return err
			}
			Logger.Info("Ingress directory created successfully", "path", serverConfig.IngressPath)
			return nil
		}
		Logger.Error("Error checking ingress directory", "path", serverConfig.IngressPath, "error", err)
		return err
	}

	// Check if it's actually a directory
	if !ingressInfo.IsDir() {
		Logger.Error("Ingress path exists but is not a directory", "path", serverConfig.IngressPath)
		return fmt.Errorf("ingress path is not a directory: %s", serverConfig.IngressPath)
	}

	Logger.Info("Ingress directory exists", "path", serverConfig.IngressPath)
	return nil
}

// documentDirectoryChecks ensures the document storage directory exists
func documentDirectoryChecks(serverConfig config.ServerConfig) error {
	if serverConfig.DocumentPath == "" {
		Logger.Warn("Document path not configured")
		return nil
	}

	// Check if directory exists
	docInfo, err := os.Stat(serverConfig.DocumentPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create the directory
			Logger.Info("Creating document directory", "path", serverConfig.DocumentPath)
			err = os.MkdirAll(serverConfig.DocumentPath, 0755)
			if err != nil {
				Logger.Error("Failed to create document directory", "path", serverConfig.DocumentPath, "error", err)
				return err
			}
			Logger.Info("Document directory created successfully", "path", serverConfig.DocumentPath)
			return nil
		}
		Logger.Error("Error checking document directory", "path", serverConfig.DocumentPath, "error", err)
		return err
	}

	// Check if it's actually a directory
	if !docInfo.IsDir() {
		Logger.Error("Document path exists but is not a directory", "path", serverConfig.DocumentPath)
		return fmt.Errorf("document path is not a directory: %s", serverConfig.DocumentPath)
	}

	Logger.Info("Document directory exists", "path", serverConfig.DocumentPath)
	return nil
}
