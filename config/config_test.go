package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckExecutables_ValidPath(t *testing.T) {
	tempDir := t.TempDir()
	validExe := filepath.Join(tempDir, "tesseract")

	file, err := os.Create(validExe)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	file.Close()

	err = os.Chmod(validExe, 0755)
	if err != nil {
		t.Fatalf("Failed to chmod file: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	err = checkExecutables(validExe, logger)
	if err != nil {
		t.Errorf("Expected no error with valid path, got: %v", err)
	}
}

func TestCheckExecutables_InvalidPath(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	invalidPath := "/nonexistent/path/to/tesseract"
	err := checkExecutables(invalidPath, logger)
	if err == nil {
		t.Error("Expected error with invalid path, got nil")
	}
	t.Logf("Correctly returned error for invalid path: %v", err)
}
