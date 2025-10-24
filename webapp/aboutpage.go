package webapp

import (
	"encoding/json"
	"fmt"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// AboutInfo represents the about information from the API
type AboutInfo struct {
	Version       string `json:"version"`
	OCRConfigured bool   `json:"ocrConfigured"`
	OCRPath       string `json:"ocrPath"`
	DatabaseType  string `json:"databaseType"`
	DatabaseHost  string `json:"databaseHost"`
	DatabasePort  string `json:"databasePort"`
	DatabaseName  string `json:"databaseName"`
	IsEphemeral   bool   `json:"isEphemeral"`
}

// AboutPage displays information about the application
type AboutPage struct {
	app.Compo
	aboutInfo AboutInfo
	loading   bool
	error     string
}

// OnMount is called when the component is mounted
func (a *AboutPage) OnMount(ctx app.Context) {
	a.loading = true
	a.fetchAboutInfo(ctx)
}

// fetchAboutInfo fetches the about information from the API
func (a *AboutPage) fetchAboutInfo(ctx app.Context) {
	ctx.Async(func() {
		res := app.Window().Call("fetch", "/api/about")

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

				ctx.Dispatch(func(ctx app.Context) {
					if err := json.Unmarshal([]byte(jsonStr), &a.aboutInfo); err != nil {
						a.error = fmt.Sprintf("Failed to parse response: %v", err)
					}
					a.loading = false
				})

				return nil
			}))

			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			ctx.Dispatch(func(ctx app.Context) {
				a.error = "Network error"
				a.loading = false
			})
			return nil
		}))
	})
}

// Render renders the about page
func (a *AboutPage) Render() app.UI {
	if a.loading {
		return app.Div().Class("about-page").Body(
			app.H2().Text("About goEDMS"),
			app.Div().Class("loading").Body(app.Text("Loading...")),
		)
	}

	if a.error != "" {
		return app.Div().Class("about-page").Body(
			app.H2().Text("About goEDMS"),
			app.Div().Class("error").Body(app.Text("Error: "+a.error)),
		)
	}

	return app.Div().Class("about-page").Body(
		app.H2().Text("About goEDMS"),
		app.Div().Class("about-content").Body(
			app.Div().Class("about-section").Body(
				app.H3().Text("Application Information"),
				app.Div().Class("info-grid").Body(
					a.renderInfoItem("Version", a.aboutInfo.Version),
					a.renderInfoItem("Database", a.getDatabaseDisplay()),
					a.renderInfoItem("OCR Status", a.getOCRStatus()),
				),
			),
			app.Div().Class("about-section").Body(
				app.H3().Text("Database Configuration"),
				app.Div().Class("config-details").Body(
					app.P().Body(
						app.Strong().Text("Database Type: "),
						app.Text(a.getDatabaseDisplay()),
					),
					app.P().Body(
						app.Strong().Text("Host: "),
						app.Text(a.aboutInfo.DatabaseHost),
					),
					app.P().Body(
						app.Strong().Text("Port: "),
						app.Text(a.aboutInfo.DatabasePort),
					),
					app.P().Body(
						app.Strong().Text("Database Name: "),
						app.Text(a.aboutInfo.DatabaseName),
					),
					app.P().Body(
						app.Strong().Text("Connection Type: "),
						app.Text(a.getConnectionType()),
					),
				),
			),
			app.Div().Class("about-section").Body(
				app.H3().Text("OCR Configuration"),
				app.Div().Class("config-details").Body(
					app.P().Body(
						app.Strong().Text("OCR Status: "),
						app.Text(a.getOCRStatus()),
					),
					app.If(a.aboutInfo.OCRConfigured, func() app.UI {
						return app.P().Body(
							app.Strong().Text("Tesseract Path: "),
							app.Text(a.aboutInfo.OCRPath),
						)
					}),
				),
			),
			app.Div().Class("about-section").Body(
				app.H3().Text("About goEDMS"),
				app.P().Text("goEDMS is a document management system built with Go and WebAssembly."),
				app.P().Text("It provides features for document ingestion, OCR processing, full-text search, and document organization."),
			),
		),
	)
}

// renderInfoItem creates an info item display
func (a *AboutPage) renderInfoItem(label, value string) app.UI {
	return app.Div().Class("info-item").Body(
		app.Div().Class("info-label").Body(app.Text(label)),
		app.Div().Class("info-value").Body(app.Text(value)),
	)
}

// getDatabaseDisplay returns a user-friendly database display name
func (a *AboutPage) getDatabaseDisplay() string {
	switch a.aboutInfo.DatabaseType {
	case "postgres":
		return "PostgreSQL"
	case "cockroachdb":
		return "CockroachDB"
	case "sqlite":
		return "SQLite"
	default:
		return a.aboutInfo.DatabaseType
	}
}

// getOCRStatus returns the OCR status as a user-friendly string
func (a *AboutPage) getOCRStatus() string {
	if a.aboutInfo.OCRConfigured {
		return "Enabled"
	}
	return "Disabled"
}

// getConnectionType returns the database connection type
func (a *AboutPage) getConnectionType() string {
	if a.aboutInfo.IsEphemeral {
		return "Ephemeral (Temporary, On-Disk)"
	}
	return "External (Persistent)"
}
