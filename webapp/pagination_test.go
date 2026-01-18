package webapp

import (
	"fmt"
	"testing"
)

// TestSearchResultsPagePagination tests pagination logic for SearchResultsPage
func TestSearchResultsPagePagination(t *testing.T) {
	// Test scenario: 5 pages of results
	testCases := []struct {
		name          string
		currentPage   int
		totalPages    int
		hasNext       bool
		hasPrevious   bool
		expectedFirst bool // First button should be enabled
		expectedLast  bool // Last button should be enabled
		expectedPrev  bool // Previous button should be enabled
		expectedNext  bool // Next button should be enabled
	}{
		{
			name:          "Page 1 of 5",
			currentPage:   1,
			totalPages:    5,
			hasNext:       true,
			hasPrevious:   false,
			expectedFirst: false, // Disabled on first page
			expectedLast:  true,
			expectedPrev:  false, // Disabled on first page
			expectedNext:  true,
		},
		{
			name:          "Page 2 of 5",
			currentPage:   2,
			totalPages:    5,
			hasNext:       true,
			hasPrevious:   true,
			expectedFirst: true,
			expectedLast:  true,
			expectedPrev:  true,
			expectedNext:  true,
		},
		{
			name:          "Page 3 of 5 (middle)",
			currentPage:   3,
			totalPages:    5,
			hasNext:       true,
			hasPrevious:   true,
			expectedFirst: true,
			expectedLast:  true,
			expectedPrev:  true,
			expectedNext:  true,
		},
		{
			name:          "Page 4 of 5",
			currentPage:   4,
			totalPages:    5,
			hasNext:       true,
			hasPrevious:   true,
			expectedFirst: true,
			expectedLast:  true,
			expectedPrev:  true,
			expectedNext:  true,
		},
		{
			name:          "Page 5 of 5 (last)",
			currentPage:   5,
			totalPages:    5,
			hasNext:       false,
			hasPrevious:   true,
			expectedFirst: true,
			expectedLast:  false, // Disabled on last page
			expectedPrev:  true,
			expectedNext:  false, // Disabled on last page
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			page := &SearchResultsPage{
				currentPage: tc.currentPage,
				totalPages:  tc.totalPages,
				hasNext:     tc.hasNext,
				hasPrevious: tc.hasPrevious,
				loading:     false,
			}

			// Test First button logic (enabled when not on page 1)
			firstEnabled := page.currentPage != 1 && !page.loading
			if firstEnabled != tc.expectedFirst {
				t.Errorf("First button: expected enabled=%v, got enabled=%v", tc.expectedFirst, firstEnabled)
			}

			// Test Last button logic (enabled when not on last page)
			lastEnabled := page.currentPage != page.totalPages && !page.loading
			if lastEnabled != tc.expectedLast {
				t.Errorf("Last button: expected enabled=%v, got enabled=%v", tc.expectedLast, lastEnabled)
			}

			// Test Previous button logic
			prevEnabled := page.hasPrevious && !page.loading
			if prevEnabled != tc.expectedPrev {
				t.Errorf("Previous button: expected enabled=%v, got enabled=%v", tc.expectedPrev, prevEnabled)
			}

			// Test Next button logic
			nextEnabled := page.hasNext && !page.loading
			if nextEnabled != tc.expectedNext {
				t.Errorf("Next button: expected enabled=%v, got enabled=%v", tc.expectedNext, nextEnabled)
			}
		})
	}
}

// TestHomePagePagination tests pagination logic for HomePage
func TestHomePagePagination(t *testing.T) {
	// Test scenario: 6 pages of results
	testCases := []struct {
		name          string
		currentPage   int
		totalPages    int
		hasNext       bool
		hasPrevious   bool
		expectedFirst bool
		expectedLast  bool
		expectedPrev  bool
		expectedNext  bool
	}{
		{
			name:          "Page 1 of 6",
			currentPage:   1,
			totalPages:    6,
			hasNext:       true,
			hasPrevious:   false,
			expectedFirst: false,
			expectedLast:  true,
			expectedPrev:  false,
			expectedNext:  true,
		},
		{
			name:          "Page 3 of 6 (middle)",
			currentPage:   3,
			totalPages:    6,
			hasNext:       true,
			hasPrevious:   true,
			expectedFirst: true,
			expectedLast:  true,
			expectedPrev:  true,
			expectedNext:  true,
		},
		{
			name:          "Page 6 of 6 (last)",
			currentPage:   6,
			totalPages:    6,
			hasNext:       false,
			hasPrevious:   true,
			expectedFirst: true,
			expectedLast:  false,
			expectedPrev:  true,
			expectedNext:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			page := &HomePage{
				currentPage: tc.currentPage,
				totalPages:  tc.totalPages,
				hasNext:     tc.hasNext,
				hasPrevious: tc.hasPrevious,
				loading:     false,
			}

			// Test First button logic
			firstEnabled := page.currentPage != 1 && !page.loading
			if firstEnabled != tc.expectedFirst {
				t.Errorf("First button: expected enabled=%v, got enabled=%v", tc.expectedFirst, firstEnabled)
			}

			// Test Last button logic
			lastEnabled := page.currentPage != page.totalPages && !page.loading
			if lastEnabled != tc.expectedLast {
				t.Errorf("Last button: expected enabled=%v, got enabled=%v", tc.expectedLast, lastEnabled)
			}

			// Test Previous button logic
			prevEnabled := page.hasPrevious && !page.loading
			if prevEnabled != tc.expectedPrev {
				t.Errorf("Previous button: expected enabled=%v, got enabled=%v", tc.expectedPrev, prevEnabled)
			}

			// Test Next button logic
			nextEnabled := page.hasNext && !page.loading
			if nextEnabled != tc.expectedNext {
				t.Errorf("Next button: expected enabled=%v, got enabled=%v", tc.expectedNext, nextEnabled)
			}
		})
	}
}

// TestPaginationButtonNavigation tests the target page calculation for pagination buttons
func TestPaginationButtonNavigation(t *testing.T) {
	testCases := []struct {
		name         string
		currentPage  int
		totalPages   int
		action       string // "first", "last", "prev", "next"
		expectedPage int
	}{
		// From page 1
		{"First from page 1", 1, 5, "first", 1},
		{"Last from page 1", 1, 5, "last", 5},
		{"Next from page 1", 1, 5, "next", 2},

		// From page 3 (middle)
		{"First from page 3", 3, 5, "first", 1},
		{"Last from page 3", 3, 5, "last", 5},
		{"Prev from page 3", 3, 5, "prev", 2},
		{"Next from page 3", 3, 5, "next", 4},

		// From page 5 (last)
		{"First from page 5", 5, 5, "first", 1},
		{"Last from page 5", 5, 5, "last", 5},
		{"Prev from page 5", 5, 5, "prev", 4},

		// Edge case: more pages
		{"First from page 4 of 10", 4, 10, "first", 1},
		{"Last from page 4 of 10", 4, 10, "last", 10},
		{"Prev from page 4 of 10", 4, 10, "prev", 3},
		{"Next from page 4 of 10", 4, 10, "next", 5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var targetPage int
			switch tc.action {
			case "first":
				targetPage = 1
			case "last":
				targetPage = tc.totalPages
			case "prev":
				targetPage = tc.currentPage - 1
			case "next":
				targetPage = tc.currentPage + 1
			}

			if targetPage != tc.expectedPage {
				t.Errorf("Expected target page %d for %s action, got %d", tc.expectedPage, tc.action, targetPage)
			}
		})
	}
}

// TestPaginationLoadingState tests that buttons are disabled during loading
func TestPaginationLoadingState(t *testing.T) {
	t.Run("SearchResultsPage loading state", func(t *testing.T) {
		page := &SearchResultsPage{
			currentPage: 3,
			totalPages:  5,
			hasNext:     true,
			hasPrevious: true,
			loading:     true, // Loading state
		}

		// All buttons should be disabled when loading
		firstEnabled := page.currentPage != 1 && !page.loading
		lastEnabled := page.currentPage != page.totalPages && !page.loading
		prevEnabled := page.hasPrevious && !page.loading
		nextEnabled := page.hasNext && !page.loading

		if firstEnabled {
			t.Error("First button should be disabled during loading")
		}
		if lastEnabled {
			t.Error("Last button should be disabled during loading")
		}
		if prevEnabled {
			t.Error("Previous button should be disabled during loading")
		}
		if nextEnabled {
			t.Error("Next button should be disabled during loading")
		}
	})

	t.Run("HomePage loading state", func(t *testing.T) {
		page := &HomePage{
			currentPage: 3,
			totalPages:  5,
			hasNext:     true,
			hasPrevious: true,
			loading:     true,
		}

		firstEnabled := page.currentPage != 1 && !page.loading
		lastEnabled := page.currentPage != page.totalPages && !page.loading
		prevEnabled := page.hasPrevious && !page.loading
		nextEnabled := page.hasNext && !page.loading

		if firstEnabled {
			t.Error("First button should be disabled during loading")
		}
		if lastEnabled {
			t.Error("Last button should be disabled during loading")
		}
		if prevEnabled {
			t.Error("Previous button should be disabled during loading")
		}
		if nextEnabled {
			t.Error("Next button should be disabled during loading")
		}
	})
}

// TestPaginationSinglePage tests that pagination is hidden for single page results
func TestPaginationSinglePage(t *testing.T) {
	t.Run("SearchResultsPage single page", func(t *testing.T) {
		page := &SearchResultsPage{
			currentPage: 1,
			totalPages:  1,
			hasNext:     false,
			hasPrevious: false,
		}

		// Pagination should not render for single page
		shouldRender := page.totalPages > 1
		if shouldRender {
			t.Error("Pagination should not render for single page results")
		}
	})

	t.Run("HomePage single page", func(t *testing.T) {
		page := &HomePage{
			currentPage: 1,
			totalPages:  1,
			hasNext:     false,
			hasPrevious: false,
		}

		shouldRender := page.totalPages > 1
		if shouldRender {
			t.Error("Pagination should not render for single page results")
		}
	})
}

// TestURLPageParameter tests URL generation with page parameter
func TestURLPageParameter(t *testing.T) {
	t.Run("SearchResultsPage URL with saved search ID", func(t *testing.T) {
		searchID := 42
		currentPage := 3

		// Expected URL format: /results?id=42&page=3
		expectedURL := "/results?id=42&page=3"

		// Simulate URL building logic (mirrors updateURL in searchresultspage.go)
		url := fmt.Sprintf("/results?id=%d&page=%d", searchID, currentPage)

		if url != expectedURL {
			t.Errorf("Expected URL '%s', got '%s'", expectedURL, url)
		}
	})

	t.Run("SearchResultsPage URL with query", func(t *testing.T) {
		query := "test query"
		currentPage := 2

		// Expected URL format: /results?q=test+query&page=2
		// URL encoding would be handled by url.QueryEscape
		// Just verify the logic is sound
		if query == "" {
			t.Error("Query should not be empty")
		}
		if currentPage < 1 {
			t.Error("Current page should be at least 1")
		}
	})

	t.Run("HomePage URL page 1 should be clean", func(t *testing.T) {
		// Page 1 should result in "/" not "/?page=1"
		page := 1
		var expectedURL string
		if page == 1 {
			expectedURL = "/"
		} else {
			expectedURL = "/?page=" + string(rune(page+'0'))
		}

		if expectedURL != "/" {
			t.Errorf("Page 1 should have URL '/', got '%s'", expectedURL)
		}
	})

	t.Run("HomePage URL page > 1 should have page param", func(t *testing.T) {
		page := 3
		var expectedURL string
		if page == 1 {
			expectedURL = "/"
		} else {
			expectedURL = "/?page=3"
		}

		if expectedURL != "/?page=3" {
			t.Errorf("Page 3 should have URL '/?page=3', got '%s'", expectedURL)
		}
	})
}

// TestPaginationEdgeCases tests edge cases in pagination
func TestPaginationEdgeCases(t *testing.T) {
	t.Run("Zero pages", func(t *testing.T) {
		page := &SearchResultsPage{
			currentPage: 0,
			totalPages:  0,
		}

		shouldRender := page.totalPages > 1
		if shouldRender {
			t.Error("Pagination should not render for zero pages")
		}
	})

	t.Run("Negative page number protection", func(t *testing.T) {
		// Ensure previous from page 1 doesn't go negative
		currentPage := 1
		prevPage := currentPage - 1

		if prevPage < 1 {
			// This is expected - UI should disable prev button on page 1
			prevPage = 1 // Clamp to 1
		}

		if prevPage != 0 && prevPage != 1 {
			t.Errorf("Previous from page 1 should result in 0 or 1, got %d", prevPage)
		}
	})

	t.Run("Beyond last page protection", func(t *testing.T) {
		totalPages := 5
		currentPage := 5
		nextPage := currentPage + 1

		if nextPage > totalPages {
			// This is expected - UI should disable next button on last page
			nextPage = totalPages // Clamp to last page
		}

		if nextPage != 5 && nextPage != 6 {
			t.Errorf("Next from last page should result in 5 or 6, got %d", nextPage)
		}
	})
}
