package webapp

import (
	"encoding/json"
	"fmt"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// TagWithUsage represents a tag with its usage count from the API
type TagWithUsage struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	UsageCount  int    `json:"usageCount"`
}

// TagsManagerPage displays and manages all tags in the system
type TagsManagerPage struct {
	app.Compo
	tags          []TagWithUsage
	loading       bool
	error         string
	editingID     int    // ID of tag currently being edited (0 = none)
	editName      string // Temporary name during editing
	editColor     string // Temporary color during editing
	editDesc      string // Temporary description during editing
	newName       string // New tag name input
	newColor      string // New tag color input
	newDesc       string // New tag description input
	deleteConfirm int    // ID of tag pending delete confirmation (0 = none)
	message       string // Success/info message
	messageType   string // "success" or "error"
}

// OnMount is called when the component is mounted
func (t *TagsManagerPage) OnMount(ctx app.Context) {
	t.loading = true
	t.newColor = "#3498db" // Default color for new tags
	t.fetchTags(ctx)
}

// fetchTags fetches all tags with usage counts from the API
func (t *TagsManagerPage) fetchTags(ctx app.Context) {
	ctx.Async(func() {
		url := BuildAPIURL("/api/tags/usage")
		res := app.Window().Call("fetch", url)

		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			if len(args) == 0 {
				return nil
			}
			response := args[0]

			status := response.Get("status").Int()
			if status != 200 {
				ctx.Dispatch(func(ctx app.Context) {
					t.error = fmt.Sprintf("Failed to load tags: HTTP %d", status)
					t.loading = false
					LogError(ctx, "Failed to load tags", map[string]interface{}{
						"component": "TagsManagerPage",
						"action":    "fetchTags",
						"status":    status,
					})
				})
				return nil
			}

			response.Call("json").Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
				if len(args) == 0 {
					return nil
				}
				jsonData := args[0]
				jsonStr := app.Window().Get("JSON").Call("stringify", jsonData).String()

				var tags []TagWithUsage
				ctx.Dispatch(func(ctx app.Context) {
					if err := json.Unmarshal([]byte(jsonStr), &tags); err != nil {
						t.error = fmt.Sprintf("Failed to parse tags: %v", err)
						LogError(ctx, "Failed to parse tags response", map[string]interface{}{
							"component": "TagsManagerPage",
							"action":    "fetchTags",
							"error":     err.Error(),
						})
					} else {
						t.tags = tags
						t.error = ""
					}
					t.loading = false
				})
				return nil
			}))
			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				t.error = "Network error loading tags"
				t.loading = false
				LogError(ctx, "Network error loading tags", map[string]interface{}{
					"component": "TagsManagerPage",
					"action":    "fetchTags",
				})
			})
			return nil
		}))
	})
}

// createTag creates a new tag
func (t *TagsManagerPage) createTag(ctx app.Context, e app.Event) {
	e.PreventDefault()

	if t.newName == "" {
		t.message = "Tag name is required"
		t.messageType = "error"
		return
	}

	body := fmt.Sprintf(`{"name": "%s", "color": "%s", "description": "%s"}`, t.newName, t.newColor, t.newDesc)

	ctx.Async(func() {
		opts := app.Window().Get("Object").New()
		opts.Set("method", "POST")
		opts.Set("headers", map[string]interface{}{
			"Content-Type": "application/json",
		})
		opts.Set("body", body)

		url := BuildAPIURL("/api/tags")
		res := app.Window().Call("fetch", url, opts)

		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			if len(args) == 0 {
				return nil
			}
			response := args[0]
			status := response.Get("status").Int()

			ctx.Dispatch(func(ctx app.Context) {
				if status == 201 || status == 200 {
					t.message = fmt.Sprintf("Tag '%s' created successfully", t.newName)
					t.messageType = "success"
					t.newName = ""
					t.newDesc = ""
					t.newColor = "#3498db"
					t.fetchTags(ctx)
				} else {
					t.message = "Failed to create tag"
					t.messageType = "error"
				}
			})
			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				t.message = "Network error creating tag"
				t.messageType = "error"
			})
			return nil
		}))
	})
}

// startEdit begins editing a tag
func (t *TagsManagerPage) startEdit(tag TagWithUsage) func(ctx app.Context, e app.Event) {
	return func(ctx app.Context, e app.Event) {
		e.PreventDefault()
		t.editingID = tag.ID
		t.editName = tag.Name
		t.editColor = tag.Color
		t.editDesc = tag.Description
		t.deleteConfirm = 0
	}
}

// cancelEdit cancels editing
func (t *TagsManagerPage) cancelEdit(ctx app.Context, e app.Event) {
	e.PreventDefault()
	t.editingID = 0
	t.editName = ""
	t.editColor = ""
	t.editDesc = ""
}

// saveEdit saves the edited tag
func (t *TagsManagerPage) saveEdit(ctx app.Context, e app.Event) {
	e.PreventDefault()

	if t.editName == "" {
		t.message = "Tag name is required"
		t.messageType = "error"
		return
	}

	body := fmt.Sprintf(`{"name": "%s", "color": "%s", "description": "%s"}`, t.editName, t.editColor, t.editDesc)
	tagID := t.editingID

	ctx.Async(func() {
		opts := app.Window().Get("Object").New()
		opts.Set("method", "PUT")
		opts.Set("headers", map[string]interface{}{
			"Content-Type": "application/json",
		})
		opts.Set("body", body)

		url := BuildAPIURL(fmt.Sprintf("/api/tags/%d", tagID))
		res := app.Window().Call("fetch", url, opts)

		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			if len(args) == 0 {
				return nil
			}
			response := args[0]
			status := response.Get("status").Int()

			ctx.Dispatch(func(ctx app.Context) {
				if status == 200 {
					t.message = "Tag updated successfully"
					t.messageType = "success"
					t.editingID = 0
					t.fetchTags(ctx)
				} else {
					t.message = "Failed to update tag"
					t.messageType = "error"
				}
			})
			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				t.message = "Network error updating tag"
				t.messageType = "error"
			})
			return nil
		}))
	})
}

// confirmDelete shows delete confirmation
func (t *TagsManagerPage) confirmDelete(tagID int) func(ctx app.Context, e app.Event) {
	return func(ctx app.Context, e app.Event) {
		e.PreventDefault()
		t.deleteConfirm = tagID
		t.editingID = 0
	}
}

// cancelDelete cancels delete confirmation
func (t *TagsManagerPage) cancelDelete(ctx app.Context, e app.Event) {
	e.PreventDefault()
	t.deleteConfirm = 0
}

// deleteTag deletes the tag
func (t *TagsManagerPage) deleteTag(tagID int, tagName string) func(ctx app.Context, e app.Event) {
	return func(ctx app.Context, e app.Event) {
		e.PreventDefault()

		ctx.Async(func() {
			opts := app.Window().Get("Object").New()
			opts.Set("method", "DELETE")

			url := BuildAPIURL(fmt.Sprintf("/api/tags/%d", tagID))
			res := app.Window().Call("fetch", url, opts)

			res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
				if len(args) == 0 {
					return nil
				}
				response := args[0]
				status := response.Get("status").Int()

				ctx.Dispatch(func(ctx app.Context) {
					if status == 204 || status == 200 {
						t.message = fmt.Sprintf("Tag '%s' deleted successfully", tagName)
						t.messageType = "success"
						t.deleteConfirm = 0
						t.fetchTags(ctx)
					} else {
						t.message = "Failed to delete tag"
						t.messageType = "error"
					}
				})
				return nil
			})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
				ctx.Dispatch(func(ctx app.Context) {
					t.message = "Network error deleting tag"
					t.messageType = "error"
				})
				return nil
			}))
		})
	}
}

// onInputChange handles input changes
func (t *TagsManagerPage) onNewNameChange(ctx app.Context, e app.Event) {
	t.newName = ctx.JSSrc().Get("value").String()
}

func (t *TagsManagerPage) onNewColorChange(ctx app.Context, e app.Event) {
	t.newColor = ctx.JSSrc().Get("value").String()
}

func (t *TagsManagerPage) onNewDescChange(ctx app.Context, e app.Event) {
	t.newDesc = ctx.JSSrc().Get("value").String()
}

func (t *TagsManagerPage) onEditNameChange(ctx app.Context, e app.Event) {
	t.editName = ctx.JSSrc().Get("value").String()
}

func (t *TagsManagerPage) onEditColorChange(ctx app.Context, e app.Event) {
	t.editColor = ctx.JSSrc().Get("value").String()
}

func (t *TagsManagerPage) onEditDescChange(ctx app.Context, e app.Event) {
	t.editDesc = ctx.JSSrc().Get("value").String()
}

// clearMessage clears the message
func (t *TagsManagerPage) clearMessage(ctx app.Context, e app.Event) {
	t.message = ""
	t.messageType = ""
}

// Render renders the tags manager page
func (t *TagsManagerPage) Render() app.UI {
	return app.Div().Class("tags-manager-page").Body(
		app.H2().Text("Tags Manager"),
		app.P().Class("page-description").Text("Create, edit, and delete tags. Tags in use by documents can still be deleted."),

		// Message display
		app.If(t.message != "", func() app.UI {
			msgClass := "message message-" + t.messageType
			return app.Div().Class(msgClass).Body(
				app.Text(t.message),
				app.Button().Class("message-close").OnClick(t.clearMessage).Text("×"),
			)
		}),

		// Create new tag form
		t.renderCreateForm(),

		// Loading state
		app.If(t.loading, func() app.UI {
			return app.Div().Class("loading").Text("Loading tags...")
		}),

		// Error state
		app.If(t.error != "" && !t.loading, func() app.UI {
			return app.Div().Class("error").Text(t.error)
		}),

		// Tags list
		app.If(!t.loading && t.error == "", func() app.UI {
			if len(t.tags) == 0 {
				return app.Div().Class("no-results").Text("No tags found. Create your first tag above!")
			}
			return app.Div().Class("tags-grid").Body(
				app.Range(t.tags).Slice(func(i int) app.UI {
					tag := t.tags[i]
					return t.renderTagCard(tag)
				}),
			)
		}),
	)
}

// renderCreateForm renders the create tag form
func (t *TagsManagerPage) renderCreateForm() app.UI {
	return app.Div().Class("create-tag-section").Body(
		app.H3().Text("Create New Tag"),
		app.Form().Class("create-tag-form").OnSubmit(t.createTag).Body(
			app.Div().Class("form-row").Body(
				app.Div().Class("form-group").Body(
					app.Label().Text("Name").For("new-tag-name"),
					app.Input().
						Type("text").
						ID("new-tag-name").
						Class("form-input").
						Placeholder("Enter tag name").
						Value(t.newName).
						OnInput(t.onNewNameChange).
						Required(true),
				),
				app.Div().Class("form-group form-group-color").Body(
					app.Label().Text("Color").For("new-tag-color"),
					app.Input().
						Type("color").
						ID("new-tag-color").
						Class("form-input-color").
						Value(t.newColor).
						OnInput(t.onNewColorChange),
				),
			),
			app.Div().Class("form-group").Body(
				app.Label().Text("Description (optional)").For("new-tag-desc"),
				app.Input().
					Type("text").
					ID("new-tag-desc").
					Class("form-input").
					Placeholder("Enter tag description").
					Value(t.newDesc).
					OnInput(t.onNewDescChange),
			),
			app.Button().Type("submit").Class("btn btn-primary").Text("Create Tag"),
		),
	)
}

// renderTagCard renders a single tag card
func (t *TagsManagerPage) renderTagCard(tag TagWithUsage) app.UI {
	isEditing := t.editingID == tag.ID
	isDeleting := t.deleteConfirm == tag.ID
	textColor := getContrastTextColor(tag.Color)

	// Delete confirmation overlay
	if isDeleting {
		return app.Div().Class("tag-card tag-card-confirm").Body(
			app.Div().Class("confirm-overlay").Body(
				app.P().Class("confirm-message").Body(
					app.Text("Delete tag '"),
					app.Strong().Text(tag.Name),
					app.Text("'?"),
				),
				app.If(tag.UsageCount > 0, func() app.UI {
					return app.P().Class("confirm-warning").Text(
						fmt.Sprintf("Warning: This tag is used by %d document(s). They will be untagged.", tag.UsageCount),
					)
				}),
				app.Div().Class("confirm-actions").Body(
					app.Button().Class("btn btn-danger").OnClick(t.deleteTag(tag.ID, tag.Name)).Text("Delete"),
					app.Button().Class("btn btn-secondary").OnClick(t.cancelDelete).Text("Cancel"),
				),
			),
		)
	}

	// Edit mode
	if isEditing {
		return app.Div().Class("tag-card tag-card-editing").Body(
			app.Div().Class("tag-edit-form").Body(
				app.Div().Class("form-row").Body(
					app.Div().Class("form-group").Body(
						app.Label().Text("Name"),
						app.Input().
							Type("text").
							Class("form-input").
							Value(t.editName).
							OnInput(t.onEditNameChange),
					),
					app.Div().Class("form-group form-group-color").Body(
						app.Label().Text("Color"),
						app.Input().
							Type("color").
							Class("form-input-color").
							Value(t.editColor).
							OnInput(t.onEditColorChange),
					),
				),
				app.Div().Class("form-group").Body(
					app.Label().Text("Description"),
					app.Input().
						Type("text").
						Class("form-input").
						Value(t.editDesc).
						OnInput(t.onEditDescChange),
				),
				app.Div().Class("edit-actions").Body(
					app.Button().Class("btn btn-primary").OnClick(t.saveEdit).Text("Save"),
					app.Button().Class("btn btn-secondary").OnClick(t.cancelEdit).Text("Cancel"),
				),
			),
		)
	}

	// Normal display mode
	return app.Div().Class("tag-card").Body(
		app.Div().Class("tag-header").Body(
			app.Span().
				Class("tag-preview").
				Style("background-color", tag.Color).
				Style("color", textColor).
				Text(tag.Name),
			app.Span().Class("tag-usage").Text(fmt.Sprintf("%d document(s)", tag.UsageCount)),
		),
		app.If(tag.Description != "", func() app.UI {
			return app.P().Class("tag-description").Text(tag.Description)
		}),
		app.Div().Class("tag-actions").Body(
			app.Button().Class("btn btn-small").OnClick(t.startEdit(tag)).Text("Edit"),
			app.Button().Class("btn btn-small btn-danger").OnClick(t.confirmDelete(tag.ID)).Text("Delete"),
		),
	)
}
