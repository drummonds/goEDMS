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
	IngressPath   string `json:"ingressPath"`
	DocumentPath  string `json:"documentPath"`
	LogLevel      string `json:"logLevel"`
	SchemaVersion string `json:"schemaVersion"`
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
		res := app.Window().Call("fetch", BuildAPIURL("/api/about"))

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
						LogError(ctx, "Failed to parse about info", map[string]interface{}{
							"component": "AboutPage",
							"action":    "fetchAboutInfo",
							"error":     err.Error(),
						})
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
				LogError(ctx, "Network error loading about info", map[string]interface{}{
					"component": "AboutPage",
					"action":    "fetchAboutInfo",
				})
			})
			return nil
		}))
	})
}

// Render renders the about page
func (a *AboutPage) Render() app.UI {
	if a.loading {
		return app.Div().Class("about-page").Body(
			app.H2().Text("About godocs"),
			app.Div().Class("loading").Body(app.Text("Loading...")),
		)
	}

	if a.error != "" {
		return app.Div().Class("about-page").Body(
			app.H2().Text("About godocs"),
			app.Div().Class("error").Body(app.Text("Error: "+a.error)),
		)
	}

	return app.Div().Class("about-page").Body(
		app.H2().Text("About godocs"),
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
				app.Div().Class("info-grid").Body(
					a.renderInfoItem("Database Type", a.getDatabaseDisplay()),
					a.renderInfoItem("Host", a.aboutInfo.DatabaseHost),
					a.renderInfoItem("Port", a.aboutInfo.DatabasePort),
					a.renderInfoItem("Database Name", a.aboutInfo.DatabaseName),
					a.renderInfoItem("Connection Type", a.getConnectionType()),
					a.renderInfoItem("Schema Version", a.aboutInfo.SchemaVersion),
				),
			),
			app.Div().Class("about-section").Body(
				app.H3().Text("OCR Configuration"),
				app.Div().Class("info-grid").Body(
					a.renderInfoItem("OCR Status", a.getOCRStatus()),
					app.If(a.aboutInfo.OCRConfigured, func() app.UI {
						return a.renderInfoItem("Tesseract Path", a.aboutInfo.OCRPath)
					}),
				),
			),
			app.Div().Class("about-section").Body(
				app.H3().Text("Document Storage"),
				app.Div().Class("info-grid").Body(
					a.renderInfoItem("Storage Path", a.aboutInfo.DocumentPath),
					a.renderInfoItem("Ingestion Folder", a.aboutInfo.IngressPath),
				),
			),
			app.Div().Class("about-section").Body(
				app.H3().Text("API & Logging"),
				app.Div().Class("info-grid").Body(
					a.renderInfoItem("Log Level", a.aboutInfo.LogLevel),
					a.renderLinkItem("API Docs", "Swagger UI", BuildAPIURL("/api/docs")),
					a.renderLinkItem("OpenAPI JSON", "Download", BuildAPIURL("/api/docs/swagger.json")),
					a.renderLinkItem("OpenAPI YAML", "Download", BuildAPIURL("/api/docs/openapi.yaml")),
				),
			),
			app.Div().Class("about-section").Body(
				app.H3().Text("About godocs"),
				app.P().Text("godocs is a document management system built with Go and WebAssembly."),
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

// renderLinkItem creates an info item with a link
func (a *AboutPage) renderLinkItem(label, linkText, url string) app.UI {
	return app.Div().Class("info-item").Body(
		app.Div().Class("info-label").Body(app.Text(label)),
		app.Div().Class("info-value").Body(
			app.A().Href(url).Target("_blank").Text(linkText),
		),
	)
}

// getDatabaseDisplay returns a user-friendly database display name
func (a *AboutPage) getDatabaseDisplay() string {
	switch a.aboutInfo.DatabaseType {
	case "postgres":
		return "PostgreSQL"
	case "cockroachdb":
		return "CockroachDB"
	case "pglike":
		return "pglike (SQLite)"
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

// AboutPage displays information about the application
type ManualLogPage struct {
	app.Compo
	aboutInfo AboutInfo
	loading   bool
	error     string
}

// OnMount is called when the ManualLogPage component is mounted
func (m *ManualLogPage) OnMount(ctx app.Context) {
	// Log an error message to the backend
	LogError(ctx, "ManualLogPage accessed - this is a test 404 error", map[string]interface{}{
		"component": "ManualLogPage",
		"action":    "OnMount",
		"page":      "/manuallog",
	})
}

// Render renders ManualLog page just to say we were here
func (a *ManualLogPage) Render() app.UI {
	return app.Div().Class("about-page").Body(
		app.H2().Text("ManualLogPage"),
		app.P().Text("This is just to say got here and is a demo front end page"),
	)
}
