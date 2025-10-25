package webapp

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// WordCloudPage displays a word cloud of the most frequent words
type WordCloudPage struct {
	app.Compo
	words    []WordFrequency
	metadata *WordCloudMetadata
	loading  bool
	error    string
}

// WordFrequency represents a word and its frequency
type WordFrequency struct {
	Word      string `json:"word"`
	Frequency int    `json:"frequency"`
}

// WordCloudMetadata contains metadata about the word cloud
type WordCloudMetadata struct {
	LastCalculation    string `json:"lastCalculation"`
	TotalDocsProcessed int    `json:"totalDocsProcessed"`
	TotalWordsIndexed  int    `json:"totalWordsIndexed"`
	Version            int    `json:"version"`
}

// WordCloudResponse is the API response structure
type WordCloudResponse struct {
	Words    []WordFrequency    `json:"words"`
	Metadata *WordCloudMetadata `json:"metadata"`
	Count    int                `json:"count"`
}

// OnMount is called when the component is mounted
func (w *WordCloudPage) OnMount(ctx app.Context) {
	w.loadWordCloud(ctx)
}

// Render renders the word cloud page
func (w *WordCloudPage) Render() app.UI {
	return app.Div().
		Class("wordcloud-page").
		Body(
			app.Div().Class("page-header").Body(
				app.H2().Text("Word Cloud"),
				app.P().Class("page-description").Text(
					"Visualization of the most frequent words across all documents",
				),
			),

			app.If(w.loading, func() app.UI {
				return app.Div().Class("loading").Body(
					app.P().Text("Loading word cloud..."),
				)
			}),

			app.If(!w.loading && w.error != "", func() app.UI {
				return app.Div().Class("error").Body(
					app.P().Text("Error: "+w.error),
					app.Button().
						Class("retry-button").
						Text("Retry").
						OnClick(func(ctx app.Context, e app.Event) {
							w.loadWordCloud(ctx)
						}),
				)
			}),

			app.If(!w.loading && w.error == "" && len(w.words) > 0, func() app.UI {
				return app.Div().Class("wordcloud-container").Body(
					// Metadata section
					app.If(w.metadata != nil, func() app.UI {
						return app.Div().Class("wordcloud-metadata").Body(
							app.P().Body(
								app.Text("Total Documents: "),
								app.Strong().Text(fmt.Sprintf("%d", w.metadata.TotalDocsProcessed)),
							),
							app.P().Body(
								app.Text("Unique Words: "),
								app.Strong().Text(fmt.Sprintf("%d", w.metadata.TotalWordsIndexed)),
							),
							app.If(w.metadata.LastCalculation != "", func() app.UI {
								return app.P().Body(
									app.Text("Last Updated: "),
									app.Strong().Text(w.metadata.LastCalculation),
								)
							}),
						)
					}),

					// Word cloud visualization
					app.Div().Class("wordcloud").Body(
						w.renderWordCloud(),
					),

					// Action buttons
					app.Div().Class("wordcloud-actions").Body(
						app.Button().
							Class("refresh-button").
							Text("Refresh").
							OnClick(func(ctx app.Context, e app.Event) {
								w.loadWordCloud(ctx)
							}),
						app.Button().
							Class("recalculate-button").
							Text("Recalculate Word Cloud").
							OnClick(func(ctx app.Context, e app.Event) {
								w.recalculateWordCloud(ctx)
							}),
					),
				)
			}),

			app.If(!w.loading && w.error == "" && len(w.words) == 0, func() app.UI {
				return app.Div().Class("no-data").Body(
					app.P().Text("No word cloud data available."),
					app.P().Text("Try ingesting some documents first."),
					app.Button().
						Class("recalculate-button").
						Text("Generate Word Cloud").
						OnClick(func(ctx app.Context, e app.Event) {
							w.recalculateWordCloud(ctx)
						}),
				)
			}),
		)
}

// renderWordCloud creates the visual word cloud from the words
func (w *WordCloudPage) renderWordCloud() app.UI {
	if len(w.words) == 0 {
		return app.Div().Text("No words to display")
	}

	// Calculate min and max frequencies for scaling
	minFreq := w.words[len(w.words)-1].Frequency
	maxFreq := w.words[0].Frequency

	// Create word elements with scaled font sizes
	wordElements := make([]app.UI, len(w.words))
	for i, word := range w.words {
		fontSize := w.calculateFontSize(word.Frequency, minFreq, maxFreq)
		color := w.getWordColor(i, len(w.words))

		wordElements[i] = app.Span().
			Class("word-cloud-item").
			Style("font-size", fmt.Sprintf("%.1fpx", fontSize)).
			Style("color", color).
			Style("margin", "5px 10px").
			Style("display", "inline-block").
			Style("cursor", "pointer").
			Title(fmt.Sprintf("%s: %d occurrences", word.Word, word.Frequency)).
			Text(word.Word).
			OnClick(func(ctx app.Context, e app.Event) {
				// Navigate to search page with this word
				ctx.Navigate("/search?term=" + word.Word)
			})
	}

	return app.Div().
		Class("word-cloud-words").
		Style("text-align", "center").
		Style("line-height", "2").
		Body(wordElements...)
}

// calculateFontSize scales font size based on frequency
// Maps frequency to a range between minSize and maxSize
func (w *WordCloudPage) calculateFontSize(freq, minFreq, maxFreq int) float64 {
	minSize := 12.0  // Minimum font size in pixels
	maxSize := 64.0  // Maximum font size in pixels

	if maxFreq == minFreq {
		return (minSize + maxSize) / 2
	}

	// Logarithmic scaling for better visual distribution
	ratio := math.Log(float64(freq-minFreq+1)) / math.Log(float64(maxFreq-minFreq+1))
	return minSize + (ratio * (maxSize - minSize))
}

// getWordColor returns a color for the word based on its position using OKLCH color space
// Creates a perceptually uniform heat map: blue → cyan → yellow → red
func (w *WordCloudPage) getWordColor(index, total int) string {
	if total <= 0 {
		return "#3b82f6" // default blue
	}

	// Calculate position in the heat map (0.0 = cold/blue, 1.0 = hot/red)
	position := float64(index) / float64(total)

	// OKLCH heat map parameters (perceptually uniform)
	// L (Lightness): kept constant at 0.65 for readability
	// C (Chroma): kept constant at 0.15 for consistent saturation
	// H (Hue): varies to create the heat map
	lightness := 0.65
	chroma := 0.15

	// Heat map hue progression (in degrees):
	// Blue (240°) → Cyan (200°) → Green (140°) → Yellow (90°) → Orange (50°) → Red (30°)
	var hue float64
	if position < 0.25 {
		// Blue to Cyan (240° to 200°)
		t := position / 0.25
		hue = 240 - (t * 40)
	} else if position < 0.5 {
		// Cyan to Green (200° to 140°)
		t := (position - 0.25) / 0.25
		hue = 200 - (t * 60)
	} else if position < 0.75 {
		// Green to Yellow (140° to 90°)
		t := (position - 0.5) / 0.25
		hue = 140 - (t * 50)
	} else {
		// Yellow to Red (90° to 30°)
		t := (position - 0.75) / 0.25
		hue = 90 - (t * 60)
	}

	// Convert OKLCH to RGB using CSS oklch() function
	// CSS supports oklch() natively in modern browsers
	return fmt.Sprintf("oklch(%.2f %.2f %ddeg)", lightness, chroma, int(hue))
}

// loadWordCloud fetches word cloud data from the API
func (w *WordCloudPage) loadWordCloud(ctx app.Context) {
	w.loading = true
	w.error = ""

	ctx.Async(func() {
		res := app.Window().Call("fetch", "/api/wordcloud?limit=100")

		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			if len(args) == 0 {
				return nil
			}
			response := args[0]

			if !response.Get("ok").Bool() {
				ctx.Dispatch(func(ctx app.Context) {
					w.error = fmt.Sprintf("HTTP error: %d", response.Get("status").Int())
					w.loading = false
				})
				return nil
			}

			response.Call("json").Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
				if len(args) == 0 {
					return nil
				}

				jsonData := args[0]
				jsonStr := app.Window().Get("JSON").Call("stringify", jsonData).String()

				var wcResponse WordCloudResponse
				ctx.Dispatch(func(ctx app.Context) {
					if err := json.Unmarshal([]byte(jsonStr), &wcResponse); err != nil {
						w.error = fmt.Sprintf("Failed to parse response: %v", err)
					} else {
						w.words = wcResponse.Words
						w.metadata = wcResponse.Metadata

						// Sort by frequency (should already be sorted, but ensure it)
						sort.Slice(w.words, func(i, j int) bool {
							return w.words[i].Frequency > w.words[j].Frequency
						})
					}
					w.loading = false
				})

				return nil
			}))

			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				w.error = "Network error: Failed to fetch word cloud"
				w.loading = false
			})
			return nil
		}))
	})
}

// recalculateWordCloud triggers a full recalculation of the word cloud
func (w *WordCloudPage) recalculateWordCloud(ctx app.Context) {
	ctx.Async(func() {
		res := app.Window().Call("fetch", "/api/wordcloud/recalculate", map[string]interface{}{
			"method": "POST",
		})

		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			if len(args) == 0 {
				return nil
			}

			ctx.Dispatch(func(ctx app.Context) {
				// Show success message
				app.Window().Call("alert", "Word cloud recalculation started. This may take a few moments.")

				// Reload after a delay to show new data
				go func() {
					time.Sleep(5 * time.Second)
					ctx.Dispatch(func(ctx app.Context) {
						w.loadWordCloud(ctx)
					})
				}()
			})

			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				app.Window().Call("alert", "Failed to trigger recalculation")
			})
			return nil
		}))
	})
}
