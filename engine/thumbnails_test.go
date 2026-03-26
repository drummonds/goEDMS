package engine

import (
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/hum3/godocs/internal/testdocs"
)

func TestThumbnailGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping thumbnail generation test in short mode")
	}

	testCases := []struct {
		name string
		file string
	}{
		{"Empty PDF", "../testdocs/1-empty.pdf"},
		{"Single page", "../testdocs/2-hello.pdf"},
		{"Two pages", "../testdocs/3-twopage.pdf"},
		{"Five pages", "../testdocs/4-fivepage.pdf"},
	}

	if err := testdocs.Generate("../testdocs"); err != nil {
		t.Fatalf("Failed to generate test docs: %v", err)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.file); os.IsNotExist(err) {
				t.Skipf("Test file not found: %s", tc.file)
			}

			err := saveThumbnailFile(tc.file)
			if err != nil {
				t.Fatalf("Failed to generate thumbnail: %v", err)
			}

			thumbnailPath := getThumbPath(tc.file)
			if _, err := os.Stat(thumbnailPath); os.IsNotExist(err) {
				t.Errorf("Thumbnail was not created: %s", thumbnailPath)
			} else {
				t.Logf("Thumbnail created: %s", filepath.Base(thumbnailPath))
				info, _ := os.Stat(thumbnailPath)
				t.Logf("  Size: %d bytes", info.Size())
				defer os.Remove(thumbnailPath)
			}
		})
	}
}

func TestGetThumbPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/path/to/001234.orig.pdf", "/path/to/001234.thumb.png"},
		{"/path/to/000042.orig.PDF", "/path/to/000042.thumb.png"},
		{"001234.orig.pdf", "001234.thumb.png"},
		{"/nested/path/001234.orig.pdf", "/nested/path/001234.thumb.png"},
	}

	for _, tt := range tests {
		result := getThumbPath(tt.input)
		if result != tt.expected {
			t.Errorf("getThumbPath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestThumbnailSupported(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"document.pdf", true},
		{"document.PDF", true},
		{"photo.jpg", true},
		{"photo.jpeg", true},
		{"photo.png", true},
		{"scan.tif", true},
		{"scan.tiff", true},
		{"document.doc", false},
		{"document.docx", false},
		{"document.txt", false},
		{"document.rtf", false},
		// Legacy thumbnail files should not be considered supported
		{"document.tn_256.png", false},
		{"/path/to/document.tn_256.png", false},
		{"photo.tn_128.png", false},
		{"scan.tn_512.png", false},
		// Canonical thumbnail files should not be considered supported
		{"001234.thumb.png", false},
		{"/path/to/001234.thumb.png", false},
	}

	for _, tt := range tests {
		result := thumbnailSupported(tt.path)
		if result != tt.expected {
			t.Errorf("thumbnailSupported(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestIsThumbnailFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		// Legacy patterns
		{"document.tn_256.png", true},
		{"/path/to/document.tn_256.png", true},
		{"photo.tn_128.png", true},
		{"scan.tn_512.png", true},
		// Canonical patterns
		{"001234.thumb.png", true},
		{"/path/to/001234.thumb.png", true},
		// Non-thumbnails
		{"document.pdf", false},
		{"photo.png", false},
		{"file.tn_256.jpg", false}, // wrong extension
		{"document.txt", false},
	}

	for _, tt := range tests {
		result := isThumbnailFile(tt.path)
		if result != tt.expected {
			t.Errorf("isThumbnailFile(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}
