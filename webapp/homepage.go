package webapp

import (
	"encoding/json"
	"fmt"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// Document represents a document from the API
type Document struct {
	StormID      int    `json:"StormID"`
	Name         string `json:"Name"`
	Path         string `json:"Path"`
	IngressTime  string `json:"IngressTime"`
	Folder       string `json:"Folder"`
	Hash         string `json:"Hash"`
	ULID         string `json:"ULID"`
	DocumentType string `json:"DocumentType"`
	FullText     string `json:"FullText"`
	URL          string `json:"URL"`
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

// HomePage displays the latest documents with pagination
type HomePage struct {
	app.Compo
	documents   []Document
	currentPage int
	totalPages  int
	totalCount  int
	hasNext     bool
	hasPrevious bool
	loading     bool
	error       string
}

// OnMount is called when the component is mounted
func (h *HomePage) OnMount(ctx app.Context) {
	h.currentPage = 1
	h.loading = true
	h.fetchDocuments(ctx, 1)
}

// fetchDocuments fetches documents for a specific page
func (h *HomePage) fetchDocuments(ctx app.Context, page int) {
	ctx.Async(func() {
		url := fmt.Sprintf("/home?page=%d", page)
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
		h.fetchDocuments(ctx, page)
	}
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
			app.H2().Text("Latest Documents"),
			app.P().Class("page-info").Text(
				fmt.Sprintf("Showing page %d of %d (%d total documents)",
					h.currentPage, h.totalPages, h.totalCount),
			),
			content,
			h.renderPagination(),
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
			OnClick(h.onPageChange(h.currentPage - 1)).
			Body(app.Text("← Previous")),

		// Page info
		app.Span().Class("pagination-info").Body(
			app.Text(fmt.Sprintf("Page %d of %d", h.currentPage, h.totalPages)),
		),

		// Next button
		app.Button().
			Class("pagination-btn").
			Disabled(!h.hasNext || h.loading).
			OnClick(h.onPageChange(h.currentPage + 1)).
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
}

// Render renders the document card
func (d *DocumentCard) Render() app.UI {
	return app.Div().
		Class("document-card").
		Body(
			app.Div().Class("document-icon").Body(
				app.Text("📄"),
			),
			app.Div().Class("document-info").Body(
				app.H3().Text(d.Document.Name),
				app.P().
					Class("document-date").
					Text("Ingested: "+d.Document.IngressTime),
				app.A().
					Href(d.Document.URL).
					Class("document-link").
					Target("_blank").
					Body(app.Text("View Document")),
			),
		)
}
