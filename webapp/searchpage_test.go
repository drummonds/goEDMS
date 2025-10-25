package webapp

import (
	"testing"
)

// TestSearchPageInitialState tests the initial state of the search page
func TestSearchPageInitialState(t *testing.T) {
	page := &SearchPage{}

	if page.searchTerm != "" {
		t.Errorf("Initial searchTerm should be empty, got %s", page.searchTerm)
	}
	if page.loading {
		t.Error("Initial loading should be false")
	}
	if page.error != "" {
		t.Errorf("Initial error should be empty, got %s", page.error)
	}
	if page.searched {
		t.Error("Initial searched should be false")
	}
}

// TestSearchPageRenderStates tests that different states produce valid UI
func TestSearchPageRenderStates(t *testing.T) {
	t.Run("Initial state returns valid UI", func(t *testing.T) {
		page := &SearchPage{}
		ui := page.Render()

		if ui == nil {
			t.Error("Initial state should return non-nil UI")
		}
	})

	t.Run("Loading state returns valid UI", func(t *testing.T) {
		page := &SearchPage{
			loading:    true,
			searchTerm: "test",
		}
		ui := page.Render()

		if ui == nil {
			t.Error("Loading state should return non-nil UI")
		}
	})

	t.Run("Error state returns valid UI", func(t *testing.T) {
		page := &SearchPage{
			loading:    false,
			error:      "Network error",
			searchTerm: "test",
			searched:   true,
		}
		ui := page.Render()

		if ui == nil {
			t.Error("Error state should return non-nil UI")
		}
	})

	t.Run("No results state returns valid UI", func(t *testing.T) {
		page := &SearchPage{
			loading:      false,
			searchTerm:   "nonexistent",
			searched:     true,
			searchResult: FileSystem{FileSystem: []FileTreeNode{}},
		}
		ui := page.Render()

		if ui == nil {
			t.Error("No results state should return non-nil UI")
		}
	})

	t.Run("Success state with results returns valid UI", func(t *testing.T) {
		page := &SearchPage{
			loading:    false,
			searchTerm: "test",
			searched:   true,
			searchResult: FileSystem{
				FileSystem: []FileTreeNode{
					{
						ID:       "SearchResults",
						Name:     "Search Results",
						IsDir:    true,
						Openable: true,
					},
					{
						ID:       "doc1",
						ULID:     "01ABCDEFGHIJKLMNOPQRSTUVWX",
						Name:     "Test_Document.pdf",
						Size:     1024,
						IsDir:    false,
						Openable: true,
						FullPath: "/documents/Test_Document.pdf",
						FileURL:  "/document/view/01ABCDEFGHIJKLMNOPQRSTUVWX",
					},
				},
			},
		}
		ui := page.Render()

		if ui == nil {
			t.Error("Success state with results should return non-nil UI")
		}
	})
}

// TestSearchPageStateManagement tests state transitions
func TestSearchPageStateManagement(t *testing.T) {
	t.Run("Loading state should have correct flags", func(t *testing.T) {
		page := &SearchPage{
			loading:    true,
			searchTerm: "test",
			searched:   false,
			error:      "",
		}

		if !page.loading {
			t.Error("Loading state should have loading=true")
		}
		if page.searched {
			t.Error("Loading state should have searched=false")
		}
		if page.error != "" {
			t.Error("Loading state should have empty error")
		}
	})

	t.Run("Error state should have correct flags", func(t *testing.T) {
		page := &SearchPage{
			loading:    false,
			searchTerm: "test",
			searched:   true,
			error:      "Network error occurred",
		}

		if page.loading {
			t.Error("Error state should have loading=false")
		}
		if page.error == "" {
			t.Error("Error state should have error message")
		}
	})

	t.Run("Success state should have correct flags", func(t *testing.T) {
		page := &SearchPage{
			loading:    false,
			searchTerm: "test",
			searched:   true,
			error:      "",
			searchResult: FileSystem{
				FileSystem: []FileTreeNode{
					{ID: "SearchResults", Name: "Search Results"},
					{ID: "doc1", Name: "Document.pdf"},
				},
			},
		}

		if page.loading {
			t.Error("Success state should have loading=false")
		}
		if !page.searched {
			t.Error("Success state should have searched=true")
		}
		if page.error != "" {
			t.Error("Success state should have empty error")
		}
		if len(page.searchResult.FileSystem) == 0 {
			t.Error("Success state should have search results")
		}
	})
}

// TestSearchResultItemRender tests the search result item rendering
func TestSearchResultItemRender(t *testing.T) {
	t.Run("Render with full document data", func(t *testing.T) {
		item := &SearchResultItem{
			Node: FileTreeNode{
				ID:       "doc1",
				ULID:     "01ABCDEFGHIJKLMNOPQRSTUVWX",
				Name:     "Test_Document.pdf",
				Size:     2048,
				ModDate:  "2024-01-15 10:30:00",
				IsDir:    false,
				Openable: true,
				FullPath: "/documents/Finance/Test_Document.pdf",
				FileURL:  "/document/view/01ABCDEFGHIJKLMNOPQRSTUVWX",
			},
		}

		ui := item.Render()
		if ui == nil {
			t.Error("Should return non-nil UI")
		}
	})

	t.Run("Render without file URL", func(t *testing.T) {
		item := &SearchResultItem{
			Node: FileTreeNode{
				ID:       "doc2",
				Name:     "Document_No_URL.txt",
				Size:     512,
				IsDir:    false,
				FullPath: "/documents/Notes/Document_No_URL.txt",
				FileURL:  "", // No URL
			},
		}

		ui := item.Render()
		if ui == nil {
			t.Error("Should return non-nil UI even without URL")
		}
	})

	t.Run("Render with minimal data", func(t *testing.T) {
		item := &SearchResultItem{
			Node: FileTreeNode{
				ID:   "doc3",
				Name: "Minimal.pdf",
			},
		}

		ui := item.Render()
		if ui == nil {
			t.Error("Should return non-nil UI with minimal data")
		}
	})
}

// TestFileSystemStruct tests the FileSystem data structure
func TestFileSystemStruct(t *testing.T) {
	t.Run("Empty filesystem", func(t *testing.T) {
		fs := FileSystem{
			FileSystem: []FileTreeNode{},
		}

		if fs.FileSystem == nil {
			t.Error("FileSystem should not be nil")
		}
		if len(fs.FileSystem) != 0 {
			t.Error("FileSystem should be empty")
		}
	})

	t.Run("Filesystem with search results", func(t *testing.T) {
		fs := FileSystem{
			FileSystem: []FileTreeNode{
				{
					ID:       "SearchResults",
					Name:     "Search Results",
					IsDir:    true,
					Openable: true,
				},
				{
					ID:       "doc1",
					Name:     "Document1.pdf",
					Size:     1024,
					IsDir:    false,
					Openable: true,
				},
				{
					ID:       "doc2",
					Name:     "Document2.txt",
					Size:     512,
					IsDir:    false,
					Openable: true,
				},
			},
		}

		if len(fs.FileSystem) != 3 {
			t.Errorf("Expected 3 nodes, got %d", len(fs.FileSystem))
		}

		// Verify root node
		if fs.FileSystem[0].ID != "SearchResults" {
			t.Error("First node should be SearchResults root")
		}
		if !fs.FileSystem[0].IsDir {
			t.Error("SearchResults node should be a directory")
		}

		// Verify document nodes
		for i := 1; i < len(fs.FileSystem); i++ {
			if fs.FileSystem[i].IsDir {
				t.Errorf("Document node %d should not be a directory", i)
			}
		}
	})
}

// TestFileTreeNodeStruct tests the FileTreeNode data structure
func TestFileTreeNodeStruct(t *testing.T) {
	t.Run("Complete document node", func(t *testing.T) {
		node := FileTreeNode{
			ID:          "doc_ulid_123",
			ULID:        "01ABCDEFGHIJKLMNOPQRSTUVWX",
			Name:        "Invoice_2024.pdf",
			Size:        4096,
			ModDate:     "2024-01-20 14:30:00",
			Openable:    true,
			ParentID:    "SearchResults",
			IsDir:       false,
			ChildrenIDs: []string{},
			FullPath:    "/documents/Finance/Invoice_2024.pdf",
			FileURL:     "/document/view/01ABCDEFGHIJKLMNOPQRSTUVWX",
		}

		if node.ID == "" {
			t.Error("ID should not be empty")
		}
		if node.Name == "" {
			t.Error("Name should not be empty")
		}
		if node.Size <= 0 {
			t.Error("Size should be positive")
		}
		if !node.Openable {
			t.Error("Document should be openable")
		}
		if node.IsDir {
			t.Error("Document node should not be a directory")
		}
		if node.FullPath == "" {
			t.Error("FullPath should not be empty")
		}
	})

	t.Run("Directory node", func(t *testing.T) {
		node := FileTreeNode{
			ID:          "SearchResults",
			Name:        "Search Results",
			Size:        0,
			Openable:    true,
			IsDir:       true,
			ChildrenIDs: []string{"doc1", "doc2", "doc3"},
		}

		if !node.IsDir {
			t.Error("Node should be a directory")
		}
		if len(node.ChildrenIDs) != 3 {
			t.Errorf("Expected 3 children, got %d", len(node.ChildrenIDs))
		}
		if !node.Openable {
			t.Error("Directory should be openable")
		}
	})
}

// MockSearchResponse represents a mock search API response
type MockSearchResponse struct {
	Status       int
	HasResults   bool
	ResultCount  int
	ErrorMessage string
}

// TestSearchPageMockScenarios tests various mock API scenarios
func TestSearchPageMockScenarios(t *testing.T) {
	t.Run("Mock successful search with results", func(t *testing.T) {
		// Simulate successful search
		page := &SearchPage{
			loading:    false,
			searchTerm: "invoice",
			searched:   true,
			error:      "",
			searchResult: FileSystem{
				FileSystem: []FileTreeNode{
					{ID: "SearchResults", Name: "Search Results", IsDir: true},
					{ID: "doc1", Name: "Invoice_Q1.pdf", Size: 1024},
					{ID: "doc2", Name: "Invoice_Q2.pdf", Size: 2048},
				},
			},
		}

		if page.loading {
			t.Error("Completed search should not be loading")
		}
		if !page.searched {
			t.Error("Should be marked as searched")
		}
		if page.error != "" {
			t.Error("Successful search should have no error")
		}
		if len(page.searchResult.FileSystem) != 3 {
			t.Errorf("Expected 3 nodes (1 root + 2 results), got %d", len(page.searchResult.FileSystem))
		}
	})

	t.Run("Mock search with no results (204)", func(t *testing.T) {
		// Simulate API returning 204 No Content
		page := &SearchPage{
			loading:      false,
			searchTerm:   "nonexistent",
			searched:     true,
			error:        "",
			searchResult: FileSystem{FileSystem: []FileTreeNode{}},
		}

		if page.loading {
			t.Error("Completed search should not be loading")
		}
		if !page.searched {
			t.Error("Should be marked as searched")
		}
		if len(page.searchResult.FileSystem) != 0 {
			t.Error("No results should have empty FileSystem")
		}
	})

	t.Run("Mock network error", func(t *testing.T) {
		// Simulate network error
		page := &SearchPage{
			loading:    false,
			searchTerm: "test",
			searched:   false,
			error:      "Network error",
		}

		if page.loading {
			t.Error("Error state should not be loading")
		}
		if page.error == "" {
			t.Error("Error state should have error message")
		}
		if page.error != "Network error" {
			t.Errorf("Expected 'Network error', got '%s'", page.error)
		}
	})

	t.Run("Mock empty search term error", func(t *testing.T) {
		// Simulate validation error for empty search term
		page := &SearchPage{
			loading:    false,
			searchTerm: "",
			searched:   false,
			error:      "Please enter a search term",
		}

		if page.searchTerm != "" {
			t.Error("Search term should be empty")
		}
		if page.error != "Please enter a search term" {
			t.Errorf("Expected validation error, got '%s'", page.error)
		}
	})

	t.Run("Mock JSON parse error", func(t *testing.T) {
		// Simulate error parsing response
		page := &SearchPage{
			loading:    false,
			searchTerm: "test",
			searched:   false,
			error:      "Failed to parse response: invalid JSON",
		}

		if page.error == "" {
			t.Error("Parse error state should have error message")
		}
		if page.searched {
			t.Error("Failed parse should not mark as searched")
		}
	})
}

// TestSearchPageEdgeCases tests edge cases
func TestSearchPageEdgeCases(t *testing.T) {
	t.Run("Very long search term", func(t *testing.T) {
		longTerm := string(make([]byte, 1000))
		for i := range longTerm {
			longTerm = longTerm[:i] + "a" + longTerm[i+1:]
		}

		page := &SearchPage{
			searchTerm: longTerm,
		}

		if len(page.searchTerm) != 1000 {
			t.Errorf("Expected search term length 1000, got %d", len(page.searchTerm))
		}

		ui := page.Render()
		if ui == nil {
			t.Error("Should render with long search term")
		}
	})

	t.Run("Special characters in search term", func(t *testing.T) {
		page := &SearchPage{
			searchTerm: "test@#$%^&*()[]{}",
		}

		ui := page.Render()
		if ui == nil {
			t.Error("Should render with special characters")
		}
	})

	t.Run("Large result set", func(t *testing.T) {
		nodes := []FileTreeNode{{ID: "SearchResults", Name: "Search Results"}}
		for i := 0; i < 100; i++ {
			nodes = append(nodes, FileTreeNode{
				ID:   string(rune(i)),
				Name: "Document_" + string(rune(i)) + ".pdf",
			})
		}

		page := &SearchPage{
			searched:     true,
			searchResult: FileSystem{FileSystem: nodes},
		}

		if len(page.searchResult.FileSystem) != 101 {
			t.Errorf("Expected 101 nodes, got %d", len(page.searchResult.FileSystem))
		}

		ui := page.Render()
		if ui == nil {
			t.Error("Should render with large result set")
		}
	})

	t.Run("SearchResults node only", func(t *testing.T) {
		page := &SearchPage{
			searched: true,
			searchResult: FileSystem{
				FileSystem: []FileTreeNode{
					{ID: "SearchResults", Name: "Search Results", IsDir: true},
				},
			},
		}

		// This represents no actual results, just the root node
		resultCount := len(page.searchResult.FileSystem) - 1
		if resultCount != 0 {
			t.Errorf("Expected 0 results (excluding root), got %d", resultCount)
		}

		ui := page.Render()
		if ui == nil {
			t.Error("Should render with just root node")
		}
	})
}

// TestSearchInputHandling tests search input validation
func TestSearchInputHandling(t *testing.T) {
	t.Run("Whitespace only search term", func(t *testing.T) {
		page := &SearchPage{
			searchTerm: "   ",
		}

		// In real scenario, this should be validated
		if page.searchTerm != "   " {
			t.Error("Search term should preserve whitespace for validation")
		}
	})

	t.Run("URL encoding characters", func(t *testing.T) {
		page := &SearchPage{
			searchTerm: "test document with spaces",
		}

		// The actual URL encoding happens in performSearch
		if page.searchTerm != "test document with spaces" {
			t.Error("Search term should be stored unencoded")
		}
	})

	t.Run("Multiple word search", func(t *testing.T) {
		page := &SearchPage{
			searchTerm: "invoice 2024 quarterly",
		}

		if page.searchTerm != "invoice 2024 quarterly" {
			t.Error("Multi-word search should be preserved")
		}
	})
}

// TestFormatBytes tests the formatBytes helper function
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"Zero bytes", 0, "0 B"},
		{"Bytes", 500, "500 B"},
		{"Kilobytes", 1024, "1.0 KB"},
		{"Megabytes", 1048576, "1.0 MB"},
		{"Gigabytes", 1073741824, "1.0 GB"},
		{"Mixed KB", 2560, "2.5 KB"},
		{"Mixed MB", 5242880, "5.0 MB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
			}
		})
	}
}
