package engine

import (
	"path/filepath"
	"strings"

	thumbnails "github.com/drummonds/go-thumbnails"
)

// thumbnailSupportedExtensions lists file extensions that the go-thumbnails library can handle
var thumbnailSupportedExtensions = []string{".pdf", ".tif", ".tiff", ".jpg", ".jpeg", ".png"}

// isThumbnailFile returns true if the file is itself a thumbnail
// Detects both legacy (*.tn_N*.png) and canonical (*.thumb.png) patterns.
func isThumbnailFile(path string) bool {
	base := filepath.Base(path)
	if strings.HasSuffix(base, ".thumb.png") {
		return true
	}
	return strings.Contains(base, ".tn_") && strings.HasSuffix(path, ".png")
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

// saveThumbnailFile generates and saves a thumbnail for a document
func saveThumbnailFile(docPath string) error {
	outputPath := getThumbPath(docPath)
	return thumbnails.GenerateStyledAndSave(docPath, outputPath, 256, thumbnails.StyleUniform)
}

// ThumbnailSupported is the exported version of thumbnailSupported.
func ThumbnailSupported(path string) bool { return thumbnailSupported(path) }

// SaveThumbnailFile is the exported version of saveThumbnailFile.
func SaveThumbnailFile(docPath string) error { return saveThumbnailFile(docPath) }
