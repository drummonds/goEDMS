package webapp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// TextResponse represents the API response for document text
type TextResponse struct {
	ULID string `json:"ulid"`
	Name string `json:"name"`
	Text string `json:"text"`
}

// TextViewPage displays the extracted text content of a document
type TextViewPage struct {
	app.Compo
	ulid     string
	response TextResponse
	loading  bool
	error    string
}

// OnMount is called when the component is mounted
func (t *TextViewPage) OnMount(ctx app.Context) {
	path := ctx.Page().URL().Path
	// Extract ULID from path like /text/01ABC...
	if len(path) > 6 && path[:6] == "/text/" {
		t.ulid = path[6:]
	}
	t.loading = true
	t.fetchText(ctx)
}

// fetchText fetches the document text from the API
func (t *TextViewPage) fetchText(ctx app.Context) {
	url := BuildAPIURL(fmt.Sprintf("/api/document/%s/text", t.ulid))

	ctx.Async(func() {
		res := app.Window().Call("fetch", url)

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
				jsonStr := app.Window().Get("JSON").Call("stringify", jsonData).String()

				ctx.Dispatch(func(ctx app.Context) {
					t.loading = false
					if status == 200 {
						var textResp TextResponse
						if err := json.Unmarshal([]byte(jsonStr), &textResp); err != nil {
							t.error = fmt.Sprintf("Failed to parse response: %v", err)
						} else {
							t.response = textResp
						}
					} else {
						// Try to extract error message
						var errResp map[string]string
						if err := json.Unmarshal([]byte(jsonStr), &errResp); err == nil {
							if errMsg, ok := errResp["error"]; ok {
								t.error = errMsg
							} else {
								t.error = "Failed to load document text"
							}
						} else {
							t.error = "Failed to load document text"
						}
					}
				})
				return nil
			}))
			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				t.loading = false
				t.error = "Network error"
			})
			return nil
		}))
	})
}

// formatTextAsParagraphs splits text by newlines and creates paragraph elements
func (t *TextViewPage) formatTextAsParagraphs() []app.UI {
	if t.response.Text == "" {
		return []app.UI{app.P().Text("No text content available.")}
	}

	// Split by double newlines first (paragraph breaks), then handle single newlines
	// Normalize line endings
	text := strings.ReplaceAll(t.response.Text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Split by double newlines for paragraphs
	paragraphs := strings.Split(text, "\n\n")

	var elements []app.UI
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		// Within a paragraph, convert single newlines to <br> elements
		lines := strings.Split(para, "\n")
		if len(lines) == 1 {
			// Simple paragraph with no internal newlines
			elements = append(elements, app.P().Class("text-paragraph").Text(para))
		} else {
			// Paragraph with internal line breaks
			var lineElements []app.UI
			for i, line := range lines {
				lineElements = append(lineElements, app.Text(line))
				if i < len(lines)-1 {
					lineElements = append(lineElements, app.Br())
				}
			}
			elements = append(elements, app.P().Class("text-paragraph").Body(lineElements...))
		}
	}

	if len(elements) == 0 {
		return []app.UI{app.P().Text("No text content available.")}
	}

	return elements
}

// Render renders the text view page
func (t *TextViewPage) Render() app.UI {
	if t.loading {
		return app.Div().Class("text-view-page").Body(
			app.H2().Text("Loading Document Text..."),
			app.Div().Class("loading").Body(app.Text("Loading...")),
		)
	}

	if t.error != "" {
		return app.Div().Class("text-view-page").Body(
			app.H2().Text("Error"),
			app.Div().Class("error-message").Body(
				app.Text(t.error),
			),
			app.P().Body(
				app.A().Href("/").Text("Back to Home"),
			),
		)
	}

	// Calculate word and character counts
	wordCount := len(strings.Fields(t.response.Text))
	charCount := len(t.response.Text)

	return app.Div().Class("text-view-page").Body(
		// Header with document info
		app.Div().Class("text-view-header").Body(
			app.H2().Text(t.response.Name),
			app.Div().Class("text-view-meta").Body(
				app.Span().Class("meta-item").Body(
					app.Strong().Text("Characters: "),
					app.Text(fmt.Sprintf("%d", charCount)),
				),
				app.Span().Class("meta-item").Body(
					app.Strong().Text("Words: "),
					app.Text(fmt.Sprintf("~%d", wordCount)),
				),
				app.A().
					Href("/edit/"+t.ulid).
					Class("meta-link").
					Text("Back to Edit Page"),
			),
		),

		// Text content
		app.Div().Class("text-view-content").Body(
			t.formatTextAsParagraphs()...,
		),
	)
}
