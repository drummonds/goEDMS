package webapp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// getContrastTextColor returns "white" or "black" based on the background color's luminance
// Uses the relative luminance formula from WCAG 2.0
func getContrastTextColor(hexColor string) string {
	// Remove # prefix if present
	hexColor = strings.TrimPrefix(hexColor, "#")

	// Default to black text if color is invalid
	if len(hexColor) != 6 && len(hexColor) != 3 {
		return "black"
	}

	// Expand 3-char hex to 6-char
	if len(hexColor) == 3 {
		hexColor = string(hexColor[0]) + string(hexColor[0]) +
			string(hexColor[1]) + string(hexColor[1]) +
			string(hexColor[2]) + string(hexColor[2])
	}

	// Parse RGB values
	r, err1 := strconv.ParseInt(hexColor[0:2], 16, 64)
	g, err2 := strconv.ParseInt(hexColor[2:4], 16, 64)
	b, err3 := strconv.ParseInt(hexColor[4:6], 16, 64)

	if err1 != nil || err2 != nil || err3 != nil {
		return "black"
	}

	// Calculate relative luminance using sRGB formula
	// Convert to 0-1 range and apply gamma correction
	rLinear := float64(r) / 255.0
	gLinear := float64(g) / 255.0
	bLinear := float64(b) / 255.0

	// Apply gamma correction
	if rLinear <= 0.03928 {
		rLinear = rLinear / 12.92
	} else {
		rLinear = ((rLinear + 0.055) / 1.055)
		rLinear = rLinear * rLinear // simplified power of 2.4
	}
	if gLinear <= 0.03928 {
		gLinear = gLinear / 12.92
	} else {
		gLinear = ((gLinear + 0.055) / 1.055)
		gLinear = gLinear * gLinear
	}
	if bLinear <= 0.03928 {
		bLinear = bLinear / 12.92
	} else {
		bLinear = ((bLinear + 0.055) / 1.055)
		bLinear = bLinear * bLinear
	}

	// Calculate luminance
	luminance := 0.2126*rLinear + 0.7152*gLinear + 0.0722*bLinear

	// Use white text for dark backgrounds (luminance < 0.5)
	if luminance < 0.5 {
		return "white"
	}
	return "black"
}

// Tag represents a tag from the API
type Tag struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Color       string  `json:"color"`
	Description string  `json:"description"`
	TagGroup    *string `json:"tag_group"`
	SortOrder   int     `json:"sort_order"`
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

// DocumentStatus represents status information for a document
type DocumentStatus struct {
	ULID          string `json:"ulid"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	DocumentType  string `json:"documentType"`
	HasThumbnail  bool   `json:"hasThumbnail"`
	ThumbnailURL  string `json:"thumbnailURL,omitempty"`
	HasText       bool   `json:"hasText"`
	TextLength    int    `json:"textLength"`
	TextURL       string `json:"textURL,omitempty"`
	HasTags       bool   `json:"hasTags"`
	TagCount      int    `json:"tagCount"`
	ViewURL       string `json:"viewURL"`
	IngressTime   string `json:"ingressTime"`
	FileExists    bool   `json:"fileExists"`
	FileSizeBytes int64  `json:"fileSizeBytes,omitempty"`
}

// EditPage allows editing document tags (unified with grouped tags for dimensions)
type EditPage struct {
	app.Compo
	ulid                  string
	document              Document
	documentStatus        DocumentStatus
	allTags               []Tag
	documentTags          []Tag
	loading               bool
	error                 string
	newTagName            string
	regeneratingThumbnail bool
	thumbnailMessage      string
}

// OnMount is called when the component is mounted
func (e *EditPage) OnMount(ctx app.Context) {
	e.ulid = ctx.Page().URL().Path[len("/edit/"):]
	e.loading = true
	e.loadData(ctx)
}

// loadData loads all necessary data for the edit page
func (e *EditPage) loadData(ctx app.Context) {
	ctx.Async(func() {
		// Load document status
		e.fetchDocumentStatus(ctx)

		// Load all tags (including grouped tags which replace dimensions)
		e.fetchAllTags(ctx)

		// Load document tags
		e.fetchDocumentTags(ctx)

		ctx.Dispatch(func(ctx app.Context) {
			e.loading = false
		})
	})
}

// fetchDocumentStatus fetches status information for this document
func (e *EditPage) fetchDocumentStatus(ctx app.Context) {
	url := BuildAPIURL(fmt.Sprintf("/api/document/%s/status", e.ulid))
	res := app.Window().Call("fetch", url)

	res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
		if len(args) == 0 {
			return nil
		}
		response := args[0]

		status := response.Get("status").Int()
		if status != 200 {
			ctx.Dispatch(func(ctx app.Context) {
				LogError(ctx, "Failed to load document status", map[string]interface{}{
					"component": "EditPage",
					"action":    "fetchDocumentStatus",
					"ulid":      e.ulid,
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

			var docStatus DocumentStatus
			ctx.Dispatch(func(ctx app.Context) {
				if err := json.Unmarshal([]byte(jsonStr), &docStatus); err != nil {
					LogError(ctx, "Failed to parse document status", map[string]interface{}{
						"component": "EditPage",
						"action":    "fetchDocumentStatus",
						"ulid":      e.ulid,
						"error":     err.Error(),
					})
				} else {
					e.documentStatus = docStatus
				}
			})
			return nil
		}))
		return nil
	}))
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

		// Check HTTP status
		status := response.Get("status").Int()
		if status != 200 {
			ctx.Dispatch(func(ctx app.Context) {
				e.error = fmt.Sprintf("Failed to load tags: HTTP %d", status)
				LogError(ctx, "Failed to load all tags", map[string]interface{}{
					"component": "EditPage",
					"action":    "fetchAllTags",
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

			var tags []Tag
			ctx.Dispatch(func(ctx app.Context) {
				if err := json.Unmarshal([]byte(jsonStr), &tags); err != nil {
					e.error = fmt.Sprintf("Failed to parse tags: %v", err)
					LogError(ctx, "Failed to parse all tags", map[string]interface{}{
						"component": "EditPage",
						"action":    "fetchAllTags",
						"error":     err.Error(),
					})
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

		// Check HTTP status
		status := response.Get("status").Int()
		if status != 200 {
			ctx.Dispatch(func(ctx app.Context) {
				e.error = fmt.Sprintf("Failed to load document tags: HTTP %d - Document ULID '%s' not found or invalid", status, e.ulid)
				LogError(ctx, "Failed to load document tags", map[string]interface{}{
					"component": "EditPage",
					"action":    "fetchDocumentTags",
					"ulid":      e.ulid,
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

			var tags []Tag
			ctx.Dispatch(func(ctx app.Context) {
				if err := json.Unmarshal([]byte(jsonStr), &tags); err != nil {
					e.error = fmt.Sprintf("Failed to parse document tags: %v", err)
					LogError(ctx, "Failed to parse document tags", map[string]interface{}{
						"component": "EditPage",
						"action":    "fetchDocumentTags",
						"ulid":      e.ulid,
						"error":     err.Error(),
					})
				} else {
					e.documentTags = tags
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
				// Reload all tags (in case tag properties changed) and document tags
				e.fetchAllTags(ctx)
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
				// Reload all tags (in case tag properties changed) and document tags
				e.fetchAllTags(ctx)
				e.fetchDocumentTags(ctx)
			})
			return nil
		}))
	})
}

// regenerateThumbnail triggers thumbnail regeneration for the document
func (e *EditPage) regenerateThumbnail(ctx app.Context, ev app.Event) {
	ev.PreventDefault()

	e.regeneratingThumbnail = true
	e.thumbnailMessage = ""

	url := BuildAPIURL(fmt.Sprintf("/api/document/%s/thumbnail/regenerate", e.ulid))

	ctx.Async(func() {
		opts := app.Window().Get("Object").New()
		opts.Set("method", "POST")

		res := app.Window().Call("fetch", url, opts)
		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			if len(args) == 0 {
				return nil
			}
			response := args[0]
			status := response.Get("status").Int()

			response.Call("json").Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
				if len(args) == 0 {
					return nil
				}
				jsonData := args[0]

				ctx.Dispatch(func(ctx app.Context) {
					e.regeneratingThumbnail = false
					if status == 200 {
						e.thumbnailMessage = "Thumbnail regenerated successfully!"
						// Reload status to update thumbnail info
						e.fetchDocumentStatus(ctx)
					} else {
						errorMsg := jsonData.Get("error").String()
						e.thumbnailMessage = "Error: " + errorMsg
					}
				})
				return nil
			}))
			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				e.regeneratingThumbnail = false
				e.thumbnailMessage = "Network error"
			})
			return nil
		}))
	})
}

// Render renders the edit page
func (e *EditPage) Render() app.UI {
	if e.loading {
		return app.Div().Class("edit-page loading").Body(
			app.H2().Text("Loading Document..."),
			app.P().Text("ULID: "+e.ulid),
		)
	}

	if e.error != "" {
		return app.Div().Class("edit-page error").Body(
			app.H2().Text("Error Loading Document"),
			app.Div().Class("error-message").Body(
				app.Text(e.error),
			),
			app.P().Text("Attempted ULID: "+e.ulid),
			app.A().Href("/").Text("← Back to Home"),
		)
	}

	return app.Div().Class("edit-page").Body(
		app.H2().Text("Edit Document: "+e.ulid),

		app.Div().Class("edit-layout").Body(
			// Left sidebar - Status and Tags (unified with grouped tags for dimensions)
			app.Div().Class("edit-sidebar").Body(
				e.renderStatusSection(),
				e.renderTagsSection(),
			),

			// Right side - Document viewer
			app.Div().Class("edit-viewer").Body(
				e.renderDocumentViewer(),
			),
		),
	)
}

// renderStatusSection renders the document status information
func (e *EditPage) renderStatusSection() app.UI {
	// Helper to format file size
	formatSize := func(bytes int64) string {
		const unit = 1024
		if bytes < unit {
			return fmt.Sprintf("%d B", bytes)
		}
		div, exp := int64(unit), 0
		for n := bytes / unit; n >= unit; n /= unit {
			div *= unit
			exp++
		}
		return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
	}

	// Status indicator helper
	statusIndicator := func(hasIt bool, label string) app.UI {
		icon := "✗"
		className := "status-indicator status-no"
		if hasIt {
			icon = "✓"
			className = "status-indicator status-yes"
		}
		return app.Span().Class(className).Body(
			app.Text(icon + " " + label),
		)
	}

	var statusItems []app.UI

	// Document name and type
	if e.documentStatus.Name != "" {
		statusItems = append(statusItems,
			app.Div().Class("status-item").Body(
				app.Strong().Text("Name: "),
				app.Text(e.documentStatus.Name),
			),
		)
	}

	if e.documentStatus.DocumentType != "" {
		statusItems = append(statusItems,
			app.Div().Class("status-item").Body(
				app.Strong().Text("Type: "),
				app.Text(e.documentStatus.DocumentType),
			),
		)
	}

	// File exists and size
	statusItems = append(statusItems,
		app.Div().Class("status-item").Body(
			statusIndicator(e.documentStatus.FileExists, "File exists"),
		),
	)

	if e.documentStatus.FileSizeBytes > 0 {
		statusItems = append(statusItems,
			app.Div().Class("status-item").Body(
				app.Strong().Text("Size: "),
				app.Text(formatSize(e.documentStatus.FileSizeBytes)),
			),
		)
	}

	// Thumbnail status with regenerate button
	var thumbnailElements []app.UI
	thumbnailElements = append(thumbnailElements, statusIndicator(e.documentStatus.HasThumbnail, "Has thumbnail"))

	if e.documentStatus.HasThumbnail && e.documentStatus.ThumbnailURL != "" {
		thumbnailElements = append(thumbnailElements,
			app.Img().
				Src(BuildAPIURL(e.documentStatus.ThumbnailURL)+"?t="+fmt.Sprint(app.Window().Get("Date").Call("now").Int())).
				Class("status-thumbnail").
				Style("max-height", "64px").
				Style("margin-left", "10px"),
		)
	}

	// Add generate/regenerate button for PDF documents
	if e.documentStatus.DocumentType == ".pdf" {
		var buttonText string
		buttonClass := "btn btn-small"
		if e.regeneratingThumbnail {
			buttonText = "Generating..."
			buttonClass += " btn-disabled"
		} else if e.documentStatus.HasThumbnail {
			buttonText = "Regenerate Thumbnail"
		} else {
			buttonText = "Generate Thumbnail"
		}
		thumbnailElements = append(thumbnailElements,
			app.Button().
				Class(buttonClass).
				Disabled(e.regeneratingThumbnail).
				OnClick(e.regenerateThumbnail).
				Text(buttonText),
		)
	}

	// Show thumbnail message if any
	if e.thumbnailMessage != "" {
		msgClass := "status-message"
		if len(e.thumbnailMessage) > 5 && e.thumbnailMessage[:5] == "Error" {
			msgClass += " status-message-error"
		} else {
			msgClass += " status-message-success"
		}
		thumbnailElements = append(thumbnailElements,
			app.Span().Class(msgClass).Text(e.thumbnailMessage),
		)
	}

	statusItems = append(statusItems,
		app.Div().Class("status-item thumbnail-item").Body(thumbnailElements...),
	)

	// Text status with link
	textUI := statusIndicator(e.documentStatus.HasText, "Has extracted text")
	if e.documentStatus.HasText {
		textUI = app.Div().Body(
			statusIndicator(true, fmt.Sprintf("Has text (%d chars)", e.documentStatus.TextLength)),
			app.A().
				Href("/text/"+e.ulid).
				Class("status-link").
				Text(" [View full text]"),
		)
	}
	statusItems = append(statusItems,
		app.Div().Class("status-item").Body(textUI),
	)

	// Tags status
	tagText := "Has tags"
	if e.documentStatus.TagCount > 0 {
		tagText = fmt.Sprintf("Has %d tag(s)", e.documentStatus.TagCount)
	}
	statusItems = append(statusItems,
		app.Div().Class("status-item").Body(
			statusIndicator(e.documentStatus.HasTags, tagText),
		),
	)

	// Ingress time
	if e.documentStatus.IngressTime != "" {
		statusItems = append(statusItems,
			app.Div().Class("status-item").Body(
				app.Strong().Text("Ingested: "),
				app.Text(e.documentStatus.IngressTime),
			),
		)
	}

	// View document link
	if e.documentStatus.ViewURL != "" {
		statusItems = append(statusItems,
			app.Div().Class("status-item").Body(
				app.A().
					Href(BuildAPIURL(e.documentStatus.ViewURL)).
					Target("_blank").
					Class("status-link").
					Text("Open document in new tab"),
			),
		)
	}

	return app.Div().Class("edit-section status-section").Body(
		app.H3().Text("Document Status"),
		app.Div().Class("status-list").Body(statusItems...),
	)
}

// renderTagsSection renders the tags editing section, organized by groups
func (e *EditPage) renderTagsSection() app.UI {
	// Separate tags into groups and free tags
	groupedTags := make(map[string][]Tag)
	var freeTags []Tag
	var groupOrder []string

	for _, tag := range e.allTags {
		if tag.TagGroup != nil && *tag.TagGroup != "" {
			group := *tag.TagGroup
			if _, exists := groupedTags[group]; !exists {
				groupOrder = append(groupOrder, group)
			}
			groupedTags[group] = append(groupedTags[group], tag)
		} else {
			freeTags = append(freeTags, tag)
		}
	}

	// Sort free tags: selected first, then unselected
	freeTags = e.sortTagsSelectedFirst(freeTags)

	var sections []app.UI

	// Render grouped tags first (one-per-group behavior)
	for _, group := range groupOrder {
		tags := groupedTags[group]
		// Sort grouped tags: selected first
		tags = e.sortTagsSelectedFirst(tags)

		sections = append(sections,
			app.Div().Class("tag-group-section").Body(
				app.Label().Class("tag-group-label").Text(group),
				app.Div().Class("tag-group-values").Body(
					app.Range(tags).Slice(func(i int) app.UI {
						tag := tags[i]
						isActive := e.hasTag(tag.ID)
						className := "tag-item tag-group-item"
						if isActive {
							className += " tag-item-active"
						} else {
							className += " tag-item-inactive"
						}

						textColor := getContrastTextColor(tag.Color)
						return app.Button().
							Class(className).
							Style("background-color", tag.Color).
							Style("color", textColor).
							OnClick(e.toggleGroupTag(tag.ID, group)).
							Body(app.Text(tag.Name))
					}),
				),
			),
		)
	}

	// Render free tags (multiple selection allowed)
	if len(freeTags) > 0 {
		sections = append(sections,
			app.Div().Class("tag-group-section tag-group-free").Body(
				app.Label().Class("tag-group-label").Text("Tags"),
				app.Div().Class("tags-list").Body(
					app.Range(freeTags).Slice(func(i int) app.UI {
						tag := freeTags[i]
						isActive := e.hasTag(tag.ID)
						className := "tag-item"
						if isActive {
							className += " tag-item-active"
						} else {
							className += " tag-item-inactive"
						}

						textColor := getContrastTextColor(tag.Color)
						return app.Button().
							Class(className).
							Style("background-color", tag.Color).
							Style("color", textColor).
							OnClick(e.toggleTag(tag.ID)).
							Body(app.Text(tag.Name))
					}),
				),
			),
		)
	}

	return app.Div().Class("edit-section").Body(
		app.H3().Text("Tags"),
		app.Div().Body(sections...),
	)
}

// sortTagsSelectedFirst sorts tags with selected ones first
func (e *EditPage) sortTagsSelectedFirst(tags []Tag) []Tag {
	var selected, unselected []Tag
	for _, tag := range tags {
		if e.hasTag(tag.ID) {
			selected = append(selected, tag)
		} else {
			unselected = append(unselected, tag)
		}
	}
	return append(selected, unselected...)
}

// toggleGroupTag handles tag selection within a group (one-per-group)
func (e *EditPage) toggleGroupTag(tagID int, group string) func(ctx app.Context, ev app.Event) {
	return func(ctx app.Context, ev app.Event) {
		ev.PreventDefault()

		// Check if this tag is already active
		if e.hasTag(tagID) {
			// Just remove it
			e.removeTag(ctx, tagID)
			return
		}

		// First, find and remove any existing tag from the same group
		for _, existingTag := range e.documentTags {
			if existingTag.TagGroup != nil && *existingTag.TagGroup == group {
				e.removeTagAndAdd(ctx, existingTag.ID, tagID)
				return
			}
		}

		// No existing tag in group, just add the new one
		e.addTag(ctx, tagID)
	}
}

// removeTagAndAdd removes one tag and adds another (for group switching)
func (e *EditPage) removeTagAndAdd(ctx app.Context, removeID int, addID int) {
	removeURL := BuildAPIURL(fmt.Sprintf("/api/documents/%s/tags/%d", e.ulid, removeID))
	addURL := BuildAPIURL(fmt.Sprintf("/api/documents/%s/tags", e.ulid))
	addBody := fmt.Sprintf(`{"tag_id": %d}`, addID)

	ctx.Async(func() {
		// Remove the old tag first
		removeOpts := app.Window().Get("Object").New()
		removeOpts.Set("method", "DELETE")

		res := app.Window().Call("fetch", removeURL, removeOpts)
		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			// Then add the new tag
			addOpts := app.Window().Get("Object").New()
			addOpts.Set("method", "POST")
			addOpts.Set("headers", map[string]interface{}{
				"Content-Type": "application/json",
			})
			addOpts.Set("body", addBody)

			addRes := app.Window().Call("fetch", addURL, addOpts)
			addRes.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
				ctx.Dispatch(func(ctx app.Context) {
					// Reload all tags (in case tag properties changed) and document tags
					e.fetchAllTags(ctx)
					e.fetchDocumentTags(ctx)
				})
				return nil
			}))
			return nil
		}))
	})
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
