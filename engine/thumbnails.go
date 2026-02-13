package engine

import (
	"path/filepath"
	"strings"

	thumbnails "github.com/drummonds/go-thumbnails"
)

// thumbnailSupportedExtensions lists file extensions that the go-thumbnails library can handle
var thumbnailSupportedExtensions = []string{".pdf", ".tif", ".tiff", ".jpg", ".jpeg", ".png"}

// isThumbnailFile returns true if the file is itself a thumbnail (matches *.tn_N*.png pattern)
func isThumbnailFile(path string) bool {
	return strings.Contains(filepath.Base(path), ".tn_") && strings.HasSuffix(path, ".png")
}

// thumbnailSupported returns true if the file extension is supported for thumbnail generation
func thumbnailSupported(path string) bool {
	if isThumbnailFile(path) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	for _, supported := range thumbnailSupportedExtensions {
		if ext == supported {
			return true
		}
	}
	return false
}

// getThumbnailPath returns the path to the thumbnail file for a document
func getThumbnailPath(docPath string) string {
	ext := filepath.Ext(docPath)
	return docPath[:len(docPath)-len(ext)] + ".tn_256.png"
}

// saveThumbnailFile generates and saves a thumbnail for a document
func saveThumbnailFile(docPath string) error {
	outputPath := getThumbnailPath(docPath)
	return thumbnails.GenerateStyledAndSave(docPath, outputPath, 256, thumbnails.StyleUniform)
}
