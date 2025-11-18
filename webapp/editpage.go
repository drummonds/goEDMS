package webapp

import (
	"encoding/json"
	"fmt"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// Tag represents a tag from the API
type Tag struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// DimensionValue represents a dimension value
type DimensionValue struct {
	ID          int    `json:"id"`
	Value       string `json:"value"`
	DisplayName string `json:"display_name"`
	Color       string `json:"color"`
}

// Dimension represents a dimension with its values
type Dimension struct {
	ID          int              `json:"id"`
	Name        string           `json:"name"`
	DisplayName string           `json:"display_name"`
	Values      []DimensionValue `json:"values"`
}

// EditPage allows editing document tags and dimensions
type EditPage struct {
	app.Compo
	ulid              string
	document          Document
	allTags           []Tag
	documentTags      []Tag
	allDimensions     []Dimension
	documentDimensions map[string]DimensionValue
	loading           bool
	error             string
	newTagName        string
}

// OnNav is called when navigating to this page
func (e *EditPage) OnNav(ctx app.Context) {
	e.ulid = ctx.Page().URL().Path[len("/edit/"):]
	e.loading = true
	e.loadData(ctx)
}

// loadData loads all necessary data for the edit page
func (e *EditPage) loadData(ctx app.Context) {
	ctx.Async(func() {
		// Fetch document details (we'll use the existing GetDocument endpoint if available)
		// For now, we'll construct a minimal document from ULID
		// In production, you'd want to add a GetDocumentByULID endpoint

		// Load all tags
		e.fetchAllTags(ctx)

		// Load document tags
		e.fetchDocumentTags(ctx)

		// Load all dimensions
		e.fetchAllDimensions(ctx)

		// Load document dimensions
		e.fetchDocumentDimensions(ctx)

		ctx.Dispatch(func(ctx app.Context) {
			e.loading = false
		})
	})
}

// fetchAllTags fetches all available tags
func (e *EditPage) fetchAllTags(ctx app.Context) {
	url := BuildAPIURL("/api/tags")
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

			var tags []Tag
			ctx.Dispatch(func(ctx app.Context) {
				if err := json.Unmarshal([]byte(jsonStr), &tags); err != nil {
					e.error = fmt.Sprintf("Failed to parse tags: %v", err)
				} else {
					e.allTags = tags
				}
			})
			return nil
		}))
		return nil
	}))
}

// fetchDocumentTags fetches tags for this document
func (e *EditPage) fetchDocumentTags(ctx app.Context) {
	url := BuildAPIURL(fmt.Sprintf("/api/documents/%s/tags", e.ulid))
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

			var tags []Tag
			ctx.Dispatch(func(ctx app.Context) {
				if err := json.Unmarshal([]byte(jsonStr), &tags); err != nil {
					e.error = fmt.Sprintf("Failed to parse document tags: %v", err)
				} else {
					e.documentTags = tags
				}
			})
			return nil
		}))
		return nil
	}))
}

// fetchAllDimensions fetches all dimensions with their values
func (e *EditPage) fetchAllDimensions(ctx app.Context) {
	url := BuildAPIURL("/api/dimensions")
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

			var dimensions []Dimension
			ctx.Dispatch(func(ctx app.Context) {
				if err := json.Unmarshal([]byte(jsonStr), &dimensions); err != nil {
					e.error = fmt.Sprintf("Failed to parse dimensions: %v", err)
				} else {
					e.allDimensions = dimensions
				}
			})
			return nil
		}))
		return nil
	}))
}

// fetchDocumentDimensions fetches dimensions for this document
func (e *EditPage) fetchDocumentDimensions(ctx app.Context) {
	url := BuildAPIURL(fmt.Sprintf("/api/documents/%s/dimensions", e.ulid))
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

			var dims map[string]DimensionValue
			ctx.Dispatch(func(ctx app.Context) {
				if err := json.Unmarshal([]byte(jsonStr), &dims); err != nil {
					e.error = fmt.Sprintf("Failed to parse document dimensions: %v", err)
				} else {
					e.documentDimensions = dims
				}
			})
			return nil
		}))
		return nil
	}))
}

// hasTag checks if document has a specific tag
func (e *EditPage) hasTag(tagID int) bool {
	for _, t := range e.documentTags {
		if t.ID == tagID {
			return true
		}
	}
	return false
}

// toggleTag adds or removes a tag from the document
func (e *EditPage) toggleTag(tagID int) func(ctx app.Context, ev app.Event) {
	return func(ctx app.Context, ev app.Event) {
		ev.PreventDefault()

		if e.hasTag(tagID) {
			// Remove tag
			e.removeTag(ctx, tagID)
		} else {
			// Add tag
			e.addTag(ctx, tagID)
		}
	}
}

// addTag adds a tag to the document
func (e *EditPage) addTag(ctx app.Context, tagID int) {
	url := BuildAPIURL(fmt.Sprintf("/api/documents/%s/tags", e.ulid))
	body := fmt.Sprintf(`{"tag_id": %d}`, tagID)

	ctx.Async(func() {
		opts := app.Window().Get("Object").New()
		opts.Set("method", "POST")
		opts.Set("headers", map[string]interface{}{
			"Content-Type": "application/json",
		})
		opts.Set("body", body)

		res := app.Window().Call("fetch", url, opts)
		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				// Reload tags
				e.fetchDocumentTags(ctx)
			})
			return nil
		}))
	})
}

// removeTag removes a tag from the document
func (e *EditPage) removeTag(ctx app.Context, tagID int) {
	url := BuildAPIURL(fmt.Sprintf("/api/documents/%s/tags/%d", e.ulid, tagID))

	ctx.Async(func() {
		opts := app.Window().Get("Object").New()
		opts.Set("method", "DELETE")

		res := app.Window().Call("fetch", url, opts)
		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				// Reload tags
				e.fetchDocumentTags(ctx)
			})
			return nil
		}))
	})
}

// setDimension sets a dimension value for the document
func (e *EditPage) setDimension(dimensionName, value string) func(ctx app.Context, ev app.Event) {
	return func(ctx app.Context, ev app.Event) {
		ev.PreventDefault()

		url := BuildAPIURL(fmt.Sprintf("/api/documents/%s/dimensions", e.ulid))
		body := fmt.Sprintf(`{"dimension_name": "%s", "value": "%s"}`, dimensionName, value)

		ctx.Async(func() {
			opts := app.Window().Get("Object").New()
			opts.Set("method", "POST")
			opts.Set("headers", map[string]interface{}{
				"Content-Type": "application/json",
			})
			opts.Set("body", body)

			res := app.Window().Call("fetch", url, opts)
			res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
				ctx.Dispatch(func(ctx app.Context) {
					// Reload dimensions
					e.fetchDocumentDimensions(ctx)
				})
				return nil
			}))
		})
	}
}

// Render renders the edit page
func (e *EditPage) Render() app.UI {
	if e.loading {
		return app.Div().Class("edit-page loading").Body(
			app.Text("Loading..."),
		)
	}

	if e.error != "" {
		return app.Div().Class("edit-page error").Body(
			app.Text("Error: " + e.error),
		)
	}

	return app.Div().Class("edit-page").Body(
		app.H2().Text("Edit Document: " + e.ulid),

		app.Div().Class("edit-layout").Body(
			// Left sidebar - Tags and Dimensions
			app.Div().Class("edit-sidebar").Body(
				e.renderTagsSection(),
				e.renderDimensionsSection(),
			),

			// Right side - Document viewer
			app.Div().Class("edit-viewer").Body(
				e.renderDocumentViewer(),
			),
		),
	)
}

// renderTagsSection renders the tags editing section
func (e *EditPage) renderTagsSection() app.UI {
	return app.Div().Class("edit-section").Body(
		app.H3().Text("Tags"),
		app.Div().Class("tags-list").Body(
			app.Range(e.allTags).Slice(func(i int) app.UI {
				tag := e.allTags[i]
				isActive := e.hasTag(tag.ID)
				className := "tag-item"
				if isActive {
					className += " tag-item-active"
				}

				return app.Button().
					Class(className).
					Style("background-color", tag.Color).
					OnClick(e.toggleTag(tag.ID)).
					Body(app.Text(tag.Name))
			}),
		),
	)
}

// renderDimensionsSection renders the dimensions editing section
func (e *EditPage) renderDimensionsSection() app.UI {
	return app.Div().Class("edit-section").Body(
		app.H3().Text("Dimensions"),
		app.Range(e.allDimensions).Slice(func(i int) app.UI {
			dim := e.allDimensions[i]

			// Get current value for this dimension
			currentValue, hasValue := e.documentDimensions[dim.Name]
			currentValueStr := ""
			if hasValue {
				currentValueStr = currentValue.Value
			}

			return app.Div().Class("dimension-group").Body(
				app.Label().Class("dimension-label").Body(
					app.Text(dim.DisplayName),
				),
				app.Div().Class("dimension-values").Body(
					app.Range(dim.Values).Slice(func(j int) app.UI {
						val := dim.Values[j]
						isActive := currentValueStr == val.Value
						className := "dimension-value"
						if isActive {
							className += " dimension-value-active"
						}

						return app.Button().
							Class(className).
							Style("background-color", val.Color).
							OnClick(e.setDimension(dim.Name, val.Value)).
							Body(app.Text(val.DisplayName))
					}),
				),
			)
		}),
	)
}

// renderDocumentViewer renders the document viewer
func (e *EditPage) renderDocumentViewer() app.UI {
	// Construct document URL
	docURL := BuildAPIURL("/document/view/" + e.ulid)

	return app.Div().Class("document-viewer-container").Body(
		app.H3().Text("Document Preview"),
		app.IFrame().
			Class("document-iframe").
			Src(docURL).
			Attr("frameborder", "0"),
	)
}
