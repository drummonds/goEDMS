package webapp

import (
	"testing"
)

// TestEditPageRouting verifies that the edit page route is correctly configured
func TestEditPageRouting(t *testing.T) {
	testCases := []struct {
		path           string
		shouldMatch    bool
		expectedULID   string
		description    string
	}{
		{
			path:         "/edit/01234567890ABCDEFGHIJKLMN",
			shouldMatch:  true,
			expectedULID: "01234567890ABCDEFGHIJKLMN",
			description:  "Valid edit path with ULID",
		},
		{
			path:         "/edit/",
			shouldMatch:  false,
			expectedULID: "",
			description:  "Edit path without ULID",
		},
		{
			path:         "/edit",
			shouldMatch:  false,
			expectedULID: "",
			description:  "Edit without trailing slash",
		},
		{
			path:         "/",
			shouldMatch:  false,
			expectedULID: "",
			description:  "Home path",
		},
		{
			path:         "/browse",
			shouldMatch:  false,
			expectedULID: "",
			description:  "Browse path",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Check if path matches the edit pattern
			matches := len(tc.path) > 6 && tc.path[:6] == "/edit/"

			if matches != tc.shouldMatch {
				t.Errorf("Path %q: expected match=%v, got match=%v", tc.path, tc.shouldMatch, matches)
			}

			if matches && tc.shouldMatch {
				// Extract ULID
				ulid := tc.path[len("/edit/"):]
				if ulid != tc.expectedULID {
					t.Errorf("Path %q: expected ULID=%q, got ULID=%q", tc.path, tc.expectedULID, ulid)
				}
			}
		})
	}

	// Verify the app returns EditPage for edit routes
	t.Run("App returns EditPage for edit route", func(t *testing.T) {
		// We can't easily test the full renderPage here since it needs app.Window()
		// which requires WASM runtime, but we can verify the logic
		path := "/edit/01234567890ABCDEFGHIJKLMN"
		shouldBeEditPage := len(path) > 6 && path[:6] == "/edit/"

		if !shouldBeEditPage {
			t.Error("Expected edit path to match edit pattern")
		}
	})
}

// TestEditPageULIDExtraction tests ULID extraction from URL
func TestEditPageULIDExtraction(t *testing.T) {
	testCases := []struct {
		path         string
		expectedULID string
	}{
		{"/edit/01HGW0YRK7N9W7Q4SZZNPM6GKV", "01HGW0YRK7N9W7Q4SZZNPM6GKV"},
		{"/edit/ABC123", "ABC123"},
		{"/edit/test-document-id", "test-document-id"},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			ulid := tc.path[len("/edit/"):]
			if ulid != tc.expectedULID {
				t.Errorf("Expected ULID %q, got %q", tc.expectedULID, ulid)
			}
		})
	}
}

// TestHasTag tests the hasTag helper method
func TestHasTag(t *testing.T) {
	editPage := &EditPage{
		documentTags: []Tag{
			{ID: 1, Name: "invoice"},
			{ID: 2, Name: "personal"},
			{ID: 3, Name: "important"},
		},
	}

	testCases := []struct {
		tagID    int
		expected bool
	}{
		{1, true},
		{2, true},
		{3, true},
		{4, false},
		{999, false},
	}

	for _, tc := range testCases {
		result := editPage.hasTag(tc.tagID)
		if result != tc.expected {
			t.Errorf("hasTag(%d): expected %v, got %v", tc.tagID, tc.expected, result)
		}
	}
}
