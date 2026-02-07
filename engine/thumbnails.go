package engine

import (
	"path/filepath"
	"strings"

	thumbnails "github.com/drummonds/go-thumbnails"
)

// thumbnailSupportedExtensions lists file extensions that the go-thumbnails library can handle
var thumbnailSupportedExtensions = []string{".pdf", ".tif", ".tiff", ".jpg", ".jpeg", ".png"}

// thumbnailSupported returns true if the file extension is supported for thumbnail generation
func thumbnailSupported(path string) bool {
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
	return docPath[:len(docPath)-len(ext)] + ".tn_64.png"
}

// saveThumbnailFile generates and saves a thumbnail for a document
func saveThumbnailFile(docPath string) error {
	outputPath := getThumbnailPath(docPath)
	return thumbnails.GenerateAndSave(docPath, outputPath, 64)
}
