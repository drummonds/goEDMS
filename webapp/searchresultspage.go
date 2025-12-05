package webapp

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// SearchResultsPage displays search results with pagination
type SearchResultsPage struct {
	app.Compo
	documents   []Document
	currentPage int
	totalPages  int
	totalCount  int
	hasNext     bool
	hasPrevious bool
	loading     bool
	error       string
	query       string
	searchName  string
	searchID    int // 0 for ad-hoc searches
}

// SearchAPIResponse matches the SearchResponse from the API
type SearchAPIResponse struct {
	Documents   []Document `json:"documents"`
	Page        int        `json:"page"`
	PageSize    int        `json:"pageSize"`
	TotalCount  int        `json:"totalCount"`
	TotalPages  int        `json:"totalPages"`
	HasNext     bool       `json:"hasNext"`
	HasPrevious bool       `json:"hasPrevious"`
	Query       string     `json:"query"`
	SearchName  string     `json:"searchName,omitempty"`
}

// OnMount is called when the component is mounted
func (s *SearchResultsPage) OnMount(ctx app.Context) {
	s.currentPage = 1
	s.loading = true

	// Parse URL parameters
	urlPath := ctx.Page().URL()
	if urlObj, err := url.Parse(urlPath.String()); err == nil {
		// Check for saved search ID
		if id := urlObj.Query().Get("id"); id != "" {
			fmt.Sscanf(id, "%d", &s.searchID)
		}
		// Check for ad-hoc query
		if q := urlObj.Query().Get("q"); q != "" {
			s.query = q
		}
		// Check for page
		if p := urlObj.Query().Get("page"); p != "" {
			fmt.Sscanf(p, "%d", &s.currentPage)
		}
	}

	s.executeSearch(ctx, s.currentPage)
}

// executeSearch fetches search results
func (s *SearchResultsPage) executeSearch(ctx app.Context, page int) {
	ctx.Async(func() {
		var searchURL string
		if s.searchID > 0 {
			// Saved search
			searchURL = BuildAPIURL(fmt.Sprintf("/api/saved-searches/%d/execute?page=%d", s.searchID, page))
		} else if s.query != "" {
			// Ad-hoc search
			encodedQuery := url.QueryEscape(s.query)
			searchURL = BuildAPIURL(fmt.Sprintf("/api/search/query?q=%s&page=%d", encodedQuery, page))
		} else {
			ctx.Dispatch(func(ctx app.Context) {
				s.error = "No search query provided"
				s.loading = false
			})
			return
		}

		res := app.Window().Call("fetch", searchURL)

		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			if len(args) == 0 {
				return nil
			}
			response := args[0]

			response.Call("json").Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
				if len(args) == 0 {
					return nil
				}

				jsonData := args[0]
				jsonStr := app.Window().Get("JSON").Call("stringify", jsonData).String()

				var resp SearchAPIResponse
				ctx.Dispatch(func(ctx app.Context) {
					if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
						s.error = fmt.Sprintf("Failed to parse response: %v", err)
					} else {
						s.documents = resp.Documents
						s.currentPage = resp.Page
						s.totalPages = resp.TotalPages
						s.totalCount = resp.TotalCount
						s.hasNext = resp.HasNext
						s.hasPrevious = resp.HasPrevious
						s.query = resp.Query
						if resp.SearchName != "" {
							s.searchName = resp.SearchName
						}

						// Store in recent searches (localStorage)
						s.addToRecentSearches(ctx)
					}
					s.loading = false
				})

				return nil
			}))

			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				s.error = "Network error"
				s.loading = false
			})
			return nil
		}))
	})
}

// RecentSearch represents a recent search for localStorage
type RecentSearch struct {
	Query      string `json:"query"`
	Name       string `json:"name"`
	SearchID   int    `json:"searchId,omitempty"`
	Timestamp  int64  `json:"timestamp"`
	TotalCount int    `json:"totalCount"`
}

// addToRecentSearches stores the current search in localStorage
func (s *SearchResultsPage) addToRecentSearches(ctx app.Context) {
	// Don't store empty or system searches
	if s.query == "" || s.query == "*" || s.query == "!untagged" {
		return
	}

	var recentSearches []RecentSearch
	ctx.LocalStorage().Get("recentSearches", &recentSearches)

	// Create new entry
	newSearch := RecentSearch{
		Query:      s.query,
		Name:       s.searchName,
		SearchID:   s.searchID,
		Timestamp:  int64(app.Window().Get("Date").Call("now").Int()),
		TotalCount: s.totalCount,
	}

	// Remove duplicates with same query
	filtered := []RecentSearch{}
	for _, rs := range recentSearches {
		if rs.Query != s.query {
			filtered = append(filtered, rs)
		}
	}

	// Add new at front
	filtered = append([]RecentSearch{newSearch}, filtered...)

	// Keep only last 10
	if len(filtered) > 10 {
		filtered = filtered[:10]
	}

	ctx.LocalStorage().Set("recentSearches", filtered)
}

// onPageChange handles page navigation
func (s *SearchResultsPage) onPageChange(page int) func(ctx app.Context, e app.Event) {
	return func(ctx app.Context, e app.Event) {
		e.PreventDefault()
		s.loading = true
		s.error = ""
		s.executeSearch(ctx, page)
	}
}

// Render renders the search results page
func (s *SearchResultsPage) Render() app.UI {
	var content app.UI

	if s.loading {
		content = app.Div().Class("loading").Body(app.Text("Searching..."))
	} else if s.error != "" {
		content = app.Div().Class("error").Body(app.Text("Error: " + s.error))
	} else if len(s.documents) == 0 {
		content = app.Div().Class("no-results").Body(app.Text("No documents found."))
	} else {
		content = app.Div().Class("document-grid").Body(
			app.Range(s.documents).Slice(func(i int) app.UI {
				doc := s.documents[i]
				return &DocumentCard{Document: doc}
			}),
		)
	}

	// Build title
	title := "Search Results"
	if s.searchName != "" {
		title = s.searchName
	}

	return app.Div().
		Class("search-results-page").
		Body(
			app.Div().Class("search-header").Body(
				app.H2().Text(title),
				app.P().Class("search-query").Body(
					app.Text("Query: "),
					app.Code().Text(s.query),
				),
			),
			app.P().Class("page-info").Text(
				fmt.Sprintf("Showing page %d of %d (%d total documents)",
					s.currentPage, s.totalPages, s.totalCount),
			),
			content,
			s.renderPagination(),
		)
}

// renderPagination renders pagination controls
func (s *SearchResultsPage) renderPagination() app.UI {
	if s.totalPages <= 1 {
		return nil
	}

	var items []app.UI

	// Previous button
	if s.hasPrevious {
		items = append(items,
			app.Button().
				Class("pagination-btn").
				Text("← Previous").
				OnClick(s.onPageChange(s.currentPage-1)),
		)
	}

	// Page numbers
	for i := 1; i <= s.totalPages; i++ {
		page := i
		class := "pagination-num"
		if i == s.currentPage {
			class += " pagination-current"
		}
		items = append(items,
			app.Button().
				Class(class).
				Text(fmt.Sprintf("%d", i)).
				OnClick(s.onPageChange(page)),
		)
	}

	// Next button
	if s.hasNext {
		items = append(items,
			app.Button().
				Class("pagination-btn").
				Text("Next →").
				OnClick(s.onPageChange(s.currentPage+1)),
		)
	}

	return app.Div().Class("pagination").Body(items...)
}
