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

// HomePage displays the latest documents
type HomePage struct {
	app.Compo
	documents []Document
	loading   bool
	error     string
}

// OnMount is called when the component is mounted
func (h *HomePage) OnMount(ctx app.Context) {
	h.loading = true
	h.fetchDocuments(ctx)
}

// fetchDocuments fetches the latest documents from the API
func (h *HomePage) fetchDocuments(ctx app.Context) {
	ctx.Async(func() {
		res := app.Window().Call("fetch", "/home")

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

				var docs []Document
				ctx.Dispatch(func(ctx app.Context) {
					if err := json.Unmarshal([]byte(jsonStr), &docs); err != nil {
						h.error = fmt.Sprintf("Failed to parse response: %v", err)
					} else {
						h.documents = docs
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

// Render renders the home page
func (h *HomePage) Render() app.UI {
	var content app.UI

	if h.loading {
		content = app.Div().Class("loading").Body(app.Text("Loading..."))
	} else if h.error != "" {
		content = app.Div().Class("error").Body(app.Text("Error: " + h.error))
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
			content,
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
