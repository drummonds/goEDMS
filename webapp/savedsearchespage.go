package webapp

import (
	"encoding/json"
	"fmt"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// SavedSearchesPage displays and manages all saved searches in the system
type SavedSearchesPage struct {
	app.Compo
	searches      []SavedSearchWithCount
	loading       bool
	error         string
	editingID     int    // ID of search currently being edited (0 = none)
	editName      string // Temporary name during editing
	editQuery     string // Temporary query during editing
	editDesc      string // Temporary description during editing
	editIcon      string // Temporary icon during editing
	newName       string // New search name input
	newQuery      string // New search query input
	newDesc       string // New search description input
	newIcon       string // New search icon input
	deleteConfirm int    // ID of search pending delete confirmation (0 = none)
	message       string // Success/info message
	messageType   string // "success" or "error"
}

// OnMount is called when the component is mounted
func (s *SavedSearchesPage) OnMount(ctx app.Context) {
	s.loading = true
	s.newIcon = "🔍" // Default icon for new searches
	s.fetchSearches(ctx)
}

// fetchSearches fetches all saved searches from the API
func (s *SavedSearchesPage) fetchSearches(ctx app.Context) {
	ctx.Async(func() {
		url := BuildAPIURL("/api/saved-searches")
		res := app.Window().Call("fetch", url)

		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			if len(args) == 0 {
				return nil
			}
			response := args[0]

			status := response.Get("status").Int()
			if status != 200 {
				ctx.Dispatch(func(ctx app.Context) {
					s.error = fmt.Sprintf("Failed to load saved searches: HTTP %d", status)
					s.loading = false
				})
				return nil
			}

			response.Call("json").Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
				if len(args) == 0 {
					return nil
				}
				jsonData := args[0]
				jsonStr := app.Window().Get("JSON").Call("stringify", jsonData).String()

				var searches []SavedSearchWithCount
				ctx.Dispatch(func(ctx app.Context) {
					if err := json.Unmarshal([]byte(jsonStr), &searches); err != nil {
						s.error = fmt.Sprintf("Failed to parse searches: %v", err)
					} else {
						s.searches = searches
						s.error = ""
					}
					s.loading = false
				})
				return nil
			}))
			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				s.error = "Network error loading saved searches"
				s.loading = false
			})
			return nil
		}))
	})
}

// savedSearchRequestBody is used for creating/updating saved searches
type savedSearchRequestBody struct {
	Name        string `json:"name"`
	Query       string `json:"query"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

// createSearch creates a new saved search
func (s *SavedSearchesPage) createSearch(ctx app.Context, e app.Event) {
	e.PreventDefault()

	if s.newName == "" {
		s.message = "Search name is required"
		s.messageType = "error"
		return
	}
	if s.newQuery == "" {
		s.message = "Search query is required"
		s.messageType = "error"
		return
	}

	reqBody := savedSearchRequestBody{
		Name:        s.newName,
		Query:       s.newQuery,
		Description: s.newDesc,
		Icon:        s.newIcon,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		s.message = "Error encoding search data"
		s.messageType = "error"
		return
	}
	body := string(bodyBytes)

	ctx.Async(func() {
		opts := app.Window().Get("Object").New()
		opts.Set("method", "POST")
		opts.Set("headers", map[string]interface{}{
			"Content-Type": "application/json",
		})
		opts.Set("body", body)

		url := BuildAPIURL("/api/saved-searches")
		res := app.Window().Call("fetch", url, opts)

		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			if len(args) == 0 {
				return nil
			}
			response := args[0]
			status := response.Get("status").Int()

			ctx.Dispatch(func(ctx app.Context) {
				if status == 201 || status == 200 {
					s.message = fmt.Sprintf("Search '%s' created successfully", s.newName)
					s.messageType = "success"
					s.newName = ""
					s.newQuery = ""
					s.newDesc = ""
					s.newIcon = "🔍"
					s.fetchSearches(ctx)
				} else {
					s.message = "Failed to create search"
					s.messageType = "error"
				}
			})
			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				s.message = "Network error creating search"
				s.messageType = "error"
			})
			return nil
		}))
	})
}

// startEdit begins editing a search
func (s *SavedSearchesPage) startEdit(search SavedSearchWithCount) func(ctx app.Context, e app.Event) {
	return func(ctx app.Context, e app.Event) {
		e.PreventDefault()
		s.editingID = search.ID
		s.editName = search.Name
		s.editQuery = search.Query
		s.editDesc = search.Description
		s.editIcon = search.Icon
		s.deleteConfirm = 0
	}
}

// cancelEdit cancels editing
func (s *SavedSearchesPage) cancelEdit(ctx app.Context, e app.Event) {
	e.PreventDefault()
	s.editingID = 0
	s.editName = ""
	s.editQuery = ""
	s.editDesc = ""
	s.editIcon = ""
}

// saveEdit saves the edited search
func (s *SavedSearchesPage) saveEdit(ctx app.Context, e app.Event) {
	e.PreventDefault()

	if s.editName == "" {
		s.message = "Search name is required"
		s.messageType = "error"
		return
	}
	if s.editQuery == "" {
		s.message = "Search query is required"
		s.messageType = "error"
		return
	}

	reqBody := savedSearchRequestBody{
		Name:        s.editName,
		Query:       s.editQuery,
		Description: s.editDesc,
		Icon:        s.editIcon,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		s.message = "Error encoding search data"
		s.messageType = "error"
		return
	}
	body := string(bodyBytes)
	searchID := s.editingID

	ctx.Async(func() {
		opts := app.Window().Get("Object").New()
		opts.Set("method", "PUT")
		opts.Set("headers", map[string]interface{}{
			"Content-Type": "application/json",
		})
		opts.Set("body", body)

		url := BuildAPIURL(fmt.Sprintf("/api/saved-searches/%d", searchID))
		res := app.Window().Call("fetch", url, opts)

		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			if len(args) == 0 {
				return nil
			}
			response := args[0]
			status := response.Get("status").Int()

			ctx.Dispatch(func(ctx app.Context) {
				if status == 200 {
					s.message = "Search updated successfully"
					s.messageType = "success"
					s.editingID = 0
					s.fetchSearches(ctx)
				} else if status == 403 {
					s.message = "Cannot modify system searches"
					s.messageType = "error"
				} else {
					s.message = "Failed to update search"
					s.messageType = "error"
				}
			})
			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				s.message = "Network error updating search"
				s.messageType = "error"
			})
			return nil
		}))
	})
}

// confirmDelete shows delete confirmation
func (s *SavedSearchesPage) confirmDelete(searchID int) func(ctx app.Context, e app.Event) {
	return func(ctx app.Context, e app.Event) {
		e.PreventDefault()
		s.deleteConfirm = searchID
		s.editingID = 0
	}
}

// cancelDelete cancels delete confirmation
func (s *SavedSearchesPage) cancelDelete(ctx app.Context, e app.Event) {
	e.PreventDefault()
	s.deleteConfirm = 0
}

// deleteSearch deletes the search
func (s *SavedSearchesPage) deleteSearch(searchID int, searchName string) func(ctx app.Context, e app.Event) {
	return func(ctx app.Context, e app.Event) {
		e.PreventDefault()

		ctx.Async(func() {
			opts := app.Window().Get("Object").New()
			opts.Set("method", "DELETE")

			url := BuildAPIURL(fmt.Sprintf("/api/saved-searches/%d", searchID))
			res := app.Window().Call("fetch", url, opts)

			res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
				if len(args) == 0 {
					return nil
				}
				response := args[0]
				status := response.Get("status").Int()

				ctx.Dispatch(func(ctx app.Context) {
					if status == 204 || status == 200 {
						s.message = fmt.Sprintf("Search '%s' deleted successfully", searchName)
						s.messageType = "success"
						s.deleteConfirm = 0
						s.fetchSearches(ctx)
					} else if status == 403 {
						s.message = "Cannot delete system searches"
						s.messageType = "error"
						s.deleteConfirm = 0
					} else {
						s.message = "Failed to delete search"
						s.messageType = "error"
					}
				})
				return nil
			})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
				ctx.Dispatch(func(ctx app.Context) {
					s.message = "Network error deleting search"
					s.messageType = "error"
				})
				return nil
			}))
		})
	}
}

// Input change handlers
func (s *SavedSearchesPage) onNewNameChange(ctx app.Context, e app.Event) {
	s.newName = ctx.JSSrc().Get("value").String()
}

func (s *SavedSearchesPage) onNewQueryChange(ctx app.Context, e app.Event) {
	s.newQuery = ctx.JSSrc().Get("value").String()
}

func (s *SavedSearchesPage) onNewDescChange(ctx app.Context, e app.Event) {
	s.newDesc = ctx.JSSrc().Get("value").String()
}

func (s *SavedSearchesPage) onNewIconChange(ctx app.Context, e app.Event) {
	s.newIcon = ctx.JSSrc().Get("value").String()
}

func (s *SavedSearchesPage) onEditNameChange(ctx app.Context, e app.Event) {
	s.editName = ctx.JSSrc().Get("value").String()
}

func (s *SavedSearchesPage) onEditQueryChange(ctx app.Context, e app.Event) {
	s.editQuery = ctx.JSSrc().Get("value").String()
}

func (s *SavedSearchesPage) onEditDescChange(ctx app.Context, e app.Event) {
	s.editDesc = ctx.JSSrc().Get("value").String()
}

func (s *SavedSearchesPage) onEditIconChange(ctx app.Context, e app.Event) {
	s.editIcon = ctx.JSSrc().Get("value").String()
}

// clearMessage clears the message
func (s *SavedSearchesPage) clearMessage(ctx app.Context, e app.Event) {
	s.message = ""
	s.messageType = ""
}

// Render renders the saved searches manager page
func (s *SavedSearchesPage) Render() app.UI {
	return app.Div().Class("saved-searches-page").Body(
		app.H2().Text("Saved Searches Manager"),
		app.P().Class("page-description").Body(
			app.Text("Create, edit, and delete saved searches. "),
			app.Strong().Text("Query syntax: "),
			app.Code().Text("text"),
			app.Text(" for text search, "),
			app.Code().Text("#tag"),
			app.Text(" to include tag, "),
			app.Code().Text("~tag"),
			app.Text(" to exclude tag, "),
			app.Code().Text("*"),
			app.Text(" for all docs, "),
			app.Code().Text("!untagged"),
			app.Text(" for inbox."),
		),

		// Message display
		app.If(s.message != "", func() app.UI {
			msgClass := "message message-" + s.messageType
			return app.Div().Class(msgClass).Body(
				app.Text(s.message),
				app.Button().Class("message-close").OnClick(s.clearMessage).Text("×"),
			)
		}),

		// Create new search form
		s.renderCreateForm(),

		// Loading state
		app.If(s.loading, func() app.UI {
			return app.Div().Class("loading").Text("Loading saved searches...")
		}),

		// Error state
		app.If(s.error != "" && !s.loading, func() app.UI {
			return app.Div().Class("error").Text(s.error)
		}),

		// Searches list
		app.If(!s.loading && s.error == "", func() app.UI {
			if len(s.searches) == 0 {
				return app.Div().Class("no-results").Text("No saved searches found. Create your first search above!")
			}
			return s.renderSearchesList()
		}),
	)
}

// renderCreateForm renders the create search form
func (s *SavedSearchesPage) renderCreateForm() app.UI {
	return app.Div().Class("create-search-section").Body(
		app.H3().Text("Create New Saved Search"),
		app.Form().Class("create-search-form").OnSubmit(s.createSearch).Body(
			app.Div().Class("form-row").Body(
				app.Div().Class("form-group").Body(
					app.Label().Text("Name").For("new-search-name"),
					app.Input().
						Type("text").
						ID("new-search-name").
						Class("form-input").
						Placeholder("e.g., Important Invoices").
						Value(s.newName).
						OnInput(s.onNewNameChange).
						Required(true),
				),
				app.Div().Class("form-group form-group-icon").Body(
					app.Label().Text("Icon").For("new-search-icon"),
					app.Input().
						Type("text").
						ID("new-search-icon").
						Class("form-input form-input-icon").
						Placeholder("🔍").
						Value(s.newIcon).
						OnInput(s.onNewIconChange).
						MaxLength(2),
				),
			),
			app.Div().Class("form-group").Body(
				app.Label().Text("Query").For("new-search-query"),
				app.Input().
					Type("text").
					ID("new-search-query").
					Class("form-input").
					Placeholder("e.g., #Invoice ~Draft").
					Value(s.newQuery).
					OnInput(s.onNewQueryChange).
					Required(true),
			),
			app.Div().Class("form-group").Body(
				app.Label().Text("Description (optional)").For("new-search-desc"),
				app.Input().
					Type("text").
					ID("new-search-desc").
					Class("form-input").
					Placeholder("Enter search description").
					Value(s.newDesc).
					OnInput(s.onNewDescChange),
			),
			app.Button().Type("submit").Class("btn btn-primary").Text("Create Search"),
		),
	)
}

// renderSearchesList renders the list of saved searches
func (s *SavedSearchesPage) renderSearchesList() app.UI {
	// Separate system and user searches
	var systemSearches []SavedSearchWithCount
	var userSearches []SavedSearchWithCount

	for _, search := range s.searches {
		if search.IsSystem {
			systemSearches = append(systemSearches, search)
		} else {
			userSearches = append(userSearches, search)
		}
	}

	var sections []app.UI

	// System searches section
	if len(systemSearches) > 0 {
		sections = append(sections,
			app.Div().Class("searches-section").Body(
				app.H3().Class("searches-section-header").Body(
					app.Span().Class("section-icon").Text("⚙️"),
					app.Text(" System Searches"),
					app.Span().Class("section-count").Text(fmt.Sprintf(" (%d)", len(systemSearches))),
				),
				app.P().Class("searches-section-note").Text("Built-in searches that cannot be modified or deleted"),
				app.Div().Class("searches-grid").Body(
					app.Range(systemSearches).Slice(func(i int) app.UI {
						return s.renderSearchCard(systemSearches[i])
					}),
				),
			),
		)
	}

	// User searches section
	sections = append(sections,
		app.Div().Class("searches-section").Body(
			app.H3().Class("searches-section-header").Body(
				app.Span().Class("section-icon").Text("🔍"),
				app.Text(" Custom Searches"),
				app.Span().Class("section-count").Text(fmt.Sprintf(" (%d)", len(userSearches))),
			),
			app.If(len(userSearches) == 0, func() app.UI {
				return app.P().Class("searches-section-note").Text("No custom searches yet. Create one above!")
			}),
			app.If(len(userSearches) > 0, func() app.UI {
				return app.Div().Class("searches-grid").Body(
					app.Range(userSearches).Slice(func(i int) app.UI {
						return s.renderSearchCard(userSearches[i])
					}),
				)
			}),
		),
	)

	return app.Div().Class("searches-sections").Body(sections...)
}

// renderSearchCard renders a single search card
func (s *SavedSearchesPage) renderSearchCard(search SavedSearchWithCount) app.UI {
	isEditing := s.editingID == search.ID
	isDeleting := s.deleteConfirm == search.ID

	// Delete confirmation overlay
	if isDeleting {
		return app.Div().Class("search-card search-card-confirm").Body(
			app.Div().Class("confirm-overlay").Body(
				app.P().Class("confirm-message").Body(
					app.Text("Delete search '"),
					app.Strong().Text(search.Name),
					app.Text("'?"),
				),
				app.Div().Class("confirm-actions").Body(
					app.Button().Class("btn btn-danger").OnClick(s.deleteSearch(search.ID, search.Name)).Text("Delete"),
					app.Button().Class("btn btn-secondary").OnClick(s.cancelDelete).Text("Cancel"),
				),
			),
		)
	}

	// Edit mode
	if isEditing {
		return app.Div().Class("search-card search-card-editing").Body(
			app.Div().Class("search-edit-form").Body(
				app.Div().Class("form-row").Body(
					app.Div().Class("form-group").Body(
						app.Label().Text("Name"),
						app.Input().
							Type("text").
							Class("form-input").
							Value(s.editName).
							OnInput(s.onEditNameChange),
					),
					app.Div().Class("form-group form-group-icon").Body(
						app.Label().Text("Icon"),
						app.Input().
							Type("text").
							Class("form-input form-input-icon").
							Value(s.editIcon).
							OnInput(s.onEditIconChange).
							MaxLength(2),
					),
				),
				app.Div().Class("form-group").Body(
					app.Label().Text("Query"),
					app.Input().
						Type("text").
						Class("form-input").
						Value(s.editQuery).
						OnInput(s.onEditQueryChange),
				),
				app.Div().Class("form-group").Body(
					app.Label().Text("Description"),
					app.Input().
						Type("text").
						Class("form-input").
						Value(s.editDesc).
						OnInput(s.onEditDescChange),
				),
				app.Div().Class("edit-actions").Body(
					app.Button().Class("btn btn-primary").OnClick(s.saveEdit).Text("Save"),
					app.Button().Class("btn btn-secondary").OnClick(s.cancelEdit).Text("Cancel"),
				),
			),
		)
	}

	// Normal display mode
	cardClass := "search-card"
	if search.IsSystem {
		cardClass += " search-card-system"
	}

	return app.Div().Class(cardClass).Body(
		app.Div().Class("search-header").Body(
			app.Span().Class("search-icon").Text(search.Icon),
			app.Span().Class("search-name").Text(search.Name),
			app.If(search.IsSystem, func() app.UI {
				return app.Span().Class("search-badge search-badge-system").Text("System")
			}),
		),
		app.Div().Class("search-query").Body(
			app.Code().Text(search.Query),
		),
		app.If(search.Description != "", func() app.UI {
			return app.P().Class("search-description").Text(search.Description)
		}),
		app.Div().Class("search-stats").Body(
			app.Span().Class("search-count").Text(fmt.Sprintf("%d document(s)", search.DocumentCount)),
		),
		app.Div().Class("search-actions").Body(
			app.A().
				Href(fmt.Sprintf("/results?id=%d", search.ID)).
				Class("btn btn-small").
				Text("Show"),
			app.If(!search.IsSystem, func() app.UI {
				return app.Div().Body(
					app.Button().Class("btn btn-small").OnClick(s.startEdit(search)).Text("Edit"),
					app.Button().Class("btn btn-small btn-danger").OnClick(s.confirmDelete(search.ID)).Text("Delete"),
				)
			}),
		),
	)
}
