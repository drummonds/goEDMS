package webapp

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// Document represents a document from the API
type Document struct {
	ID           int    `json:"ID"`
	Name         string `json:"Name"`
	Path         string `json:"Path"`
	IngressTime  string `json:"IngressTime"`
	Folder       string `json:"Folder"`
	Hash         string `json:"Hash"`
	ULID         string `json:"ULID"`
	DocumentType string `json:"DocumentType"`
	FullText     string `json:"FullText"`
	URL          string `json:"URL"`
	ThumbnailURL string `json:"thumbnailURL,omitempty"`
}

// PaginatedResponse represents the paginated API response
type PaginatedResponse struct {
	Documents   []Document `json:"documents"`
	Page        int        `json:"page"`
	PageSize    int        `json:"pageSize"`
	TotalCount  int        `json:"totalCount"`
	TotalPages  int        `json:"totalPages"`
	HasNext     bool       `json:"hasNext"`
	HasPrevious bool       `json:"hasPrevious"`
}

// SavedSearchWithCount matches the API response
type SavedSearchWithCount struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Query         string `json:"query"`
	Icon          string `json:"icon"`
	SortOrder     int    `json:"sort_order"`
	IsSystem      bool   `json:"is_system"`
	DocumentCount int    `json:"document_count"`
}

// HomePage displays the latest documents with pagination
type HomePage struct {
	app.Compo
	documents      []Document
	currentPage    int
	totalPages     int
	totalCount     int
	hasNext        bool
	hasPrevious    bool
	loading        bool
	error          string
	savedSearches  []SavedSearchWithCount
	recentSearches []RecentSearch
}

// OnMount is called when the component is mounted
func (h *HomePage) OnMount(ctx app.Context) {
	h.currentPage = 1
	h.loading = true

	// Parse URL parameters for page number
	urlPath := ctx.Page().URL()
	if urlObj, err := url.Parse(urlPath.String()); err == nil {
		if p := urlObj.Query().Get("page"); p != "" {
			fmt.Sscanf(p, "%d", &h.currentPage)
		}
	}

	h.fetchDocuments(ctx, h.currentPage)
	h.fetchSavedSearches(ctx)
	h.loadRecentSearches(ctx)
}

// fetchSavedSearches fetches saved searches from the API
func (h *HomePage) fetchSavedSearches(ctx app.Context) {
	ctx.Async(func() {
		url := BuildAPIURL("/api/saved-searches")
		res := app.Window().Call("fetch", url)

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

				var searches []SavedSearchWithCount
				ctx.Dispatch(func(ctx app.Context) {
					if err := json.Unmarshal([]byte(jsonStr), &searches); err == nil {
						h.savedSearches = searches
					}
				})
				return nil
			}))
			return nil
		}))
	})
}

// loadRecentSearches loads recent searches from localStorage
func (h *HomePage) loadRecentSearches(ctx app.Context) {
	var recentSearches []RecentSearch
	ctx.LocalStorage().Get("recentSearches", &recentSearches)
	// Keep only top 3
	if len(recentSearches) > 3 {
		recentSearches = recentSearches[:3]
	}
	h.recentSearches = recentSearches
}

// fetchDocuments fetches documents for a specific page
func (h *HomePage) fetchDocuments(ctx app.Context, page int) {
	ctx.Async(func() {
		url := BuildAPIURL(fmt.Sprintf("/api/documents/latest?page=%d", page))
		res := app.Window().Call("fetch", url)

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

				var resp PaginatedResponse
				ctx.Dispatch(func(ctx app.Context) {
					if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
						h.error = fmt.Sprintf("Failed to parse response: %v", err)
						LogError(ctx, "Failed to parse documents response", map[string]interface{}{
							"component": "HomePage",
							"action":    "fetchDocuments",
							"page":      h.currentPage,
							"error":     err.Error(),
						})
					} else {
						h.documents = resp.Documents
						h.currentPage = resp.Page
						h.totalPages = resp.TotalPages
						h.totalCount = resp.TotalCount
						h.hasNext = resp.HasNext
						h.hasPrevious = resp.HasPrevious
					}
					h.loading = false
				})

				return nil
			}))

			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				h.error = "Network error"
				h.loading = false
				LogError(ctx, "Network error loading documents", map[string]interface{}{
					"component": "HomePage",
					"action":    "fetchDocuments",
					"page":      h.currentPage,
				})
			})
			return nil
		}))
	})
}

// onPageChange handles page navigation
func (h *HomePage) onPageChange(page int) func(ctx app.Context, e app.Event) {
	return func(ctx app.Context, e app.Event) {
		e.PreventDefault()
		h.loading = true
		h.error = ""
		// Update URL with the new page number
		h.updateURL(page)
		h.fetchDocuments(ctx, page)
	}
}

// updateURL updates the browser URL with the current page number
func (h *HomePage) updateURL(page int) {
	var newURL string
	if page == 1 {
		newURL = "/"
	} else {
		newURL = fmt.Sprintf("/?page=%d", page)
	}
	// Use history.replaceState to update URL without navigation
	app.Window().Get("history").Call("replaceState", nil, "", newURL)
}

// Render renders the home page
func (h *HomePage) Render() app.UI {
	var content app.UI

	if h.loading {
		content = app.Div().Class("loading").Body(app.Text("Loading..."))
	} else if h.error != "" {
		content = app.Div().Class("error").Body(app.Text("Error: " + h.error))
	} else if len(h.documents) == 0 {
		content = app.Div().Class("no-results").Body(app.Text("No documents found."))
	} else {
		content = app.Div().Class("document-grid").Body(
			app.Range(h.documents).Slice(func(i int) app.UI {
				doc := h.documents[i]
				return &DocumentCard{Document: doc}
			}),
		)
	}

	return app.Div().
		Class("home-page").
		Body(
			h.renderSavedSearches(),
			h.renderRecentSearches(),
			app.H2().Text("Latest Documents"),
			app.P().Class("page-info").Text(
				fmt.Sprintf("Showing page %d of %d (%d total documents)",
					h.currentPage, h.totalPages, h.totalCount),
			),
			content,
			h.renderPagination(),
		)
}

// renderSavedSearches renders the saved searches section
func (h *HomePage) renderSavedSearches() app.UI {
	if len(h.savedSearches) == 0 {
		return nil
	}

	items := make([]app.UI, 0, len(h.savedSearches))
	for _, search := range h.savedSearches {
		s := search // capture for closure
		items = append(items,
			app.A().
				Class("saved-search-card").
				Href(fmt.Sprintf("/results?id=%d", s.ID)).
				Body(
					app.Span().Class("saved-search-icon").Text(s.Icon),
					app.Span().Class("saved-search-name").Text(s.Name),
					app.Span().Class("saved-search-count").Text(fmt.Sprintf("(%d)", s.DocumentCount)),
				),
		)
	}

	return app.Div().Class("saved-searches-section").Body(
		app.H3().Text("Saved Searches"),
		app.Div().Class("saved-searches-grid").Body(items...),
	)
}

// renderRecentSearches renders the recent searches section
func (h *HomePage) renderRecentSearches() app.UI {
	if len(h.recentSearches) == 0 {
		return nil
	}

	items := make([]app.UI, 0, len(h.recentSearches))
	for _, search := range h.recentSearches {
		s := search // capture for closure
		var href string
		if s.SearchID > 0 {
			href = fmt.Sprintf("/results?id=%d", s.SearchID)
		} else {
			href = fmt.Sprintf("/results?q=%s", s.Query)
		}

		// Determine icon based on query type
		icon := "🔍"
		if len(s.Query) > 0 && s.Query[0] == '#' {
			icon = "🏷️"
		}

		displayName := s.Name
		if displayName == "" {
			displayName = s.Query
		}

		items = append(items,
			app.A().
				Class("recent-search-card").
				Href(href).
				Body(
					app.Span().Class("recent-search-icon").Text(icon),
					app.Span().Class("recent-search-name").Text(displayName),
					app.Span().Class("recent-search-count").Text(fmt.Sprintf("(%d)", s.TotalCount)),
				),
		)
	}

	return app.Div().Class("recent-searches-section").Body(
		app.H3().Text("Recent Searches"),
		app.Div().Class("recent-searches-grid").Body(items...),
	)
}

// renderPagination renders the pagination controls
func (h *HomePage) renderPagination() app.UI {
	if h.totalPages <= 1 {
		return app.Div() // No pagination needed
	}

	return app.Div().Class("pagination").Body(
		// Previous button
		app.Button().
			Class("pagination-btn").
			Disabled(!h.hasPrevious || h.loading).
			OnClick(h.onPageChange(h.currentPage-1)).
			Body(app.Text("← Previous")),

		// Page info
		app.Span().Class("pagination-info").Body(
			app.Text(fmt.Sprintf("Page %d of %d", h.currentPage, h.totalPages)),
		),

		// Next button
		app.Button().
			Class("pagination-btn").
			Disabled(!h.hasNext || h.loading).
			OnClick(h.onPageChange(h.currentPage+1)).
			Body(app.Text("Next →")),

		// Jump to first/last
		app.Div().Class("pagination-jump").Body(
			app.Button().
				Class("pagination-btn-small").
				Disabled(h.currentPage == 1 || h.loading).
				OnClick(h.onPageChange(1)).
				Body(app.Text("First")),
			app.Button().
				Class("pagination-btn-small").
				Disabled(h.currentPage == h.totalPages || h.loading).
				OnClick(h.onPageChange(h.totalPages)).
				Body(app.Text("Last")),
		),
	)
}

// DocumentCard displays a single document card
type DocumentCard struct {
	app.Compo
	Document Document
	tags     []Tag
	loaded   bool
}

// OnMount is called when the component is mounted - fetches document tags
func (d *DocumentCard) OnMount(ctx app.Context) {
	d.fetchTags(ctx)
}

// fetchTags fetches tags for this document
func (d *DocumentCard) fetchTags(ctx app.Context) {
	if d.Document.ULID == "" {
		return
	}

	ctx.Async(func() {
		url := BuildAPIURL(fmt.Sprintf("/api/documents/%s/tags", d.Document.ULID))
		res := app.Window().Call("fetch", url)

		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			if len(args) == 0 {
				return nil
			}
			response := args[0]

			status := response.Get("status").Int()
			if status != 200 {
				ctx.Dispatch(func(ctx app.Context) {
					d.loaded = true
				})
				return nil
			}

			response.Call("json").Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
				if len(args) == 0 {
					return nil
				}
				jsonData := args[0]
				jsonStr := app.Window().Get("JSON").Call("stringify", jsonData).String()

				var tags []Tag
				ctx.Dispatch(func(ctx app.Context) {
					if err := json.Unmarshal([]byte(jsonStr), &tags); err == nil {
						d.tags = tags
					}
					d.loaded = true
				})
				return nil
			}))
			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				d.loaded = true
			})
			return nil
		}))
	})
}

// Render renders the document card
func (d *DocumentCard) Render() app.UI {
	// Render thumbnail or fallback icon
	var iconContent app.UI
	if d.Document.ThumbnailURL != "" {
		// Show thumbnail image if available
		iconContent = app.Img().
			Src(BuildAPIURL(d.Document.ThumbnailURL)).
			Alt("Document thumbnail").
			Class("document-thumbnail")
	} else {
		// Fallback to emoji icon
		iconContent = app.Text("📄")
	}

	// Render tags (max 5 visible)
	const maxVisibleTags = 5
	var tagsContent app.UI
	if len(d.tags) > 0 {
		visibleTags := d.tags
		hasMore := false
		if len(d.tags) > maxVisibleTags {
			visibleTags = d.tags[:maxVisibleTags]
			hasMore = true
		}

		tagElements := make([]app.UI, 0, len(visibleTags)+1)
		for _, tag := range visibleTags {
			textColor := getContrastTextColor(tag.Color)
			tagElements = append(tagElements,
				app.Span().
					Class("document-tag").
					Style("background-color", tag.Color).
					Style("color", textColor).
					Text(tag.Name),
			)
		}
		if hasMore {
			tagElements = append(tagElements,
				app.Span().Class("document-tags-more").Text(fmt.Sprintf("+%d more", len(d.tags)-maxVisibleTags)),
			)
		}
		tagsContent = app.Div().Class("document-tags").Body(tagElements...)
	}

	return app.Div().
		Class("document-card").
		Body(
			app.Div().Class("document-icon").Body(
				iconContent,
			),
			app.Div().Class("document-info").Body(
				app.H3().Text(d.Document.Name),
				app.P().
					Class("document-date").
					Text("Ingested: "+d.Document.IngressTime),
				tagsContent,
				app.Div().Class("document-actions").Body(
					app.A().
						Href(d.Document.URL).
						Class("document-link").
						Target("_blank").
						Body(app.Text("View Document")),
					app.Button().
						Class("document-link document-link-edit").
						OnClick(d.navigateToEdit).
						Body(app.Text("Edit")),
				),
			),
		)
}

// navigateToEdit handles navigation to the edit page
func (d *DocumentCard) navigateToEdit(ctx app.Context, e app.Event) {
	e.PreventDefault()
	ctx.Navigate("/edit/" + d.Document.ULID)
}
