package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestThumbnailGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping thumbnail generation test in short mode")
	}

	testCases := []struct {
		name    string
		file    string
	}{
		{"Empty PDF", "../testdocs/1-empty.pdf"},
		{"Single page", "../testdocs/2-hello.pdf"},
		{"Two pages", "../testdocs/5-twopage.pdf"},
		{"Five pages", "../testdocs/6-fivepage.pdf"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.file); os.IsNotExist(err) {
				t.Skipf("Test file not found: %s (run 'go run cmd/testgen/main.go' first)", tc.file)
			}

			err := saveThumbnailFile(tc.file)
			if err != nil {
				t.Fatalf("Failed to generate thumbnail: %v", err)
			}

			thumbnailPath := getThumbnailPath(tc.file)
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

func TestGetThumbnailPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/path/to/document.pdf", "/path/to/document.tn_64.png"},
		{"/path/to/file.PDF", "/path/to/file.tn_64.png"},
		{"document.pdf", "document.tn_64.png"},
		{"/nested/path/invoice.pdf", "/nested/path/invoice.tn_64.png"},
	}

	for _, tt := range tests {
		result := getThumbnailPath(tt.input)
		if result != tt.expected {
			t.Errorf("getThumbnailPath(%q) = %q, want %q", tt.input, result, tt.expected)
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
	}

	for _, tt := range tests {
		result := thumbnailSupported(tt.path)
		if result != tt.expected {
			t.Errorf("thumbnailSupported(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}
