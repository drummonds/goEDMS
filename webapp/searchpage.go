package webapp

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// SearchPage provides full-text search functionality
type SearchPage struct {
	app.Compo
	searchTerm   string
	searchResult FileSystem
	loading      bool
	error        string
	searched     bool
}

// Render renders the search page
func (s *SearchPage) Render() app.UI {
	var content app.UI

	if s.loading {
		content = app.Div().Class("loading").Body(app.Text("Searching..."))
	} else if s.error != "" {
		content = app.Div().Class("error").Body(app.Text("Error: " + s.error))
	} else if s.searched && len(s.searchResult.FileSystem) == 0 {
		content = app.Div().Class("no-results").Body(app.Text("No results found for: " + s.searchTerm))
	} else if s.searched && len(s.searchResult.FileSystem) > 0 {
		content = app.Div().Class("search-results").Body(
			app.H3().Text(fmt.Sprintf("Found %d results", len(s.searchResult.FileSystem)-1)),
			app.Div().Class("result-list").Body(
				app.Range(s.searchResult.FileSystem).Slice(func(i int) app.UI {
					node := s.searchResult.FileSystem[i]
					if node.ID == "SearchResults" {
						return nil
					}
					return &SearchResultItem{Node: node}
				}),
			),
		)
	}

	return app.Div().
		Class("search-page").
		Body(
			app.H2().Text("Search Documents"),
			app.Div().Class("search-form").Body(
				app.Input().
					Type("text").
					Class("search-input").
					Placeholder("Enter search term...").
					Value(s.searchTerm).
					OnInput(func(ctx app.Context, e app.Event) {
						s.searchTerm = ctx.JSSrc().Get("value").String()
					}).
					OnKeyDown(func(ctx app.Context, e app.Event) {
						if e.Get("key").String() == "Enter" {
							s.performSearch(ctx)
						}
					}),
				app.Button().
					Class("search-button").
					Text("Search").
					OnClick(func(ctx app.Context, e app.Event) {
						s.performSearch(ctx)
					}),
			),
			content,
		)
}

// performSearch executes the search
func (s *SearchPage) performSearch(ctx app.Context) {
	if s.searchTerm == "" {
		s.error = "Please enter a search term"
		return
	}

	s.loading = true
	s.error = ""
	s.searched = false

	ctx.Async(func() {
		encodedTerm := url.QueryEscape(s.searchTerm)
		searchURL := fmt.Sprintf("/search/?term=%s", encodedTerm)

		res := app.Window().Call("fetch", searchURL)

		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			if len(args) == 0 {
				return nil
			}
			response := args[0]

			if response.Get("status").Int() == 204 {
				ctx.Dispatch(func(ctx app.Context) {
					s.searchResult = FileSystem{}
					s.loading = false
					s.searched = true
				})
				return nil
			}

			response.Call("json").Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
				if len(args) == 0 {
					return nil
				}

				jsonData := args[0]
				jsonStr := app.Window().Get("JSON").Call("stringify", jsonData).String()

				var fs FileSystem
				ctx.Dispatch(func(ctx app.Context) {
					if err := json.Unmarshal([]byte(jsonStr), &fs); err != nil {
						s.error = fmt.Sprintf("Failed to parse response: %v", err)
					} else {
						s.searchResult = fs
						s.searched = true
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

// SearchResultItem displays a single search result
type SearchResultItem struct {
	app.Compo
	Node FileTreeNode
}

// Render renders the search result item
func (s *SearchResultItem) Render() app.UI {
	var nameUI app.UI
	if s.Node.FileURL != "" {
		nameUI = app.A().Href(s.Node.FileURL).Target("_blank").Text(s.Node.Name)
	} else {
		nameUI = app.Text(s.Node.Name)
	}

	var sizeUI app.UI
	if s.Node.Size > 0 {
		sizeUI = app.P().Class("result-size").Text(fmt.Sprintf("Size: %s", formatBytes(s.Node.Size)))
	}

	var dateUI app.UI
	if s.Node.ModDate != "" {
		dateUI = app.P().Class("result-date").Text(fmt.Sprintf("Modified: %s", s.Node.ModDate))
	}

	return app.Div().
		Class("search-result-item").
		Body(
			app.Div().Class("result-icon").Body(
				app.Text("📄"),
			),
			app.Div().Class("result-info").Body(
				app.H4().Body(nameUI),
				app.P().Class("result-path").Text(s.Node.FullPath),
				sizeUI,
				dateUI,
			),
		)
}
