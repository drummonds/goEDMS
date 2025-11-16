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

	// Test files
	testCases := []struct {
		name          string
		pdfFile       string
		expectedPages int
		expectPlus    bool
	}{
		{"Empty PDF", "../testdocs/1-empty.pdf", 1, false},
		{"Single page", "../testdocs/2-hello.pdf", 1, false},
		{"Two pages", "../testdocs/5-twopage.pdf", 2, false},
		{"Five pages", "../testdocs/6-fivepage.pdf", 5, true}, // Should show first 4 + "+"
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Check if test PDF exists
			if _, err := os.Stat(tc.pdfFile); os.IsNotExist(err) {
				t.Skipf("Test PDF not found: %s (run 'go run cmd/testgen/main.go' first)", tc.pdfFile)
			}

			// Generate thumbnail
			err := generateThumbnail(tc.pdfFile)
			if err != nil {
				t.Fatalf("Failed to generate thumbnail: %v", err)
			}

			// Check thumbnail exists
			thumbnailPath := getThumbnailPath(tc.pdfFile)
			if _, err := os.Stat(thumbnailPath); os.IsNotExist(err) {
				t.Errorf("Thumbnail was not created: %s", thumbnailPath)
			} else {
				t.Logf("✓ Thumbnail created: %s", filepath.Base(thumbnailPath))

				// Get file info
				info, _ := os.Stat(thumbnailPath)
				t.Logf("  Size: %d bytes", info.Size())

				// Clean up
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
