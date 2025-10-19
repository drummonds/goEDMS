package webapp

import (
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// IngestPage allows users to trigger the ingestion process manually
type IngestPage struct {
	app.Compo
	running bool
	result  string
	error   string
}

// Render renders the ingest page
func (i *IngestPage) Render() app.UI {
	buttonText := "Run Ingestion Now"
	if i.running {
		buttonText = "Running..."
	}

	return app.Div().
		Class("ingest-page").
		Body(
			app.H2().Text("Manual Ingestion"),
			app.P().Text("Click the button below to run the document ingestion process now. This will scan the ingress folder and import any new documents."),

			app.Div().Class("ingest-controls").Body(
				app.Button().
					Class("btn-primary").
					Disabled(i.running).
					OnClick(i.onIngestClick).
					Body(app.Text(buttonText)),
			),

			i.renderStatus(),
		)
}

// renderStatus renders the status section
func (i *IngestPage) renderStatus() app.UI {
	if i.running {
		return app.Div().Class("loading").Body(
			app.Text("Ingestion in progress..."),
		)
	}

	if i.error != "" {
		return app.Div().Class("error").Body(
			app.Text("Error: "+i.error),
		)
	}

	if i.result != "" {
		return app.Div().Class("success").Body(
			app.Text(i.result),
		)
	}

	return app.Div()
}

// onIngestClick handles the ingest button click
func (i *IngestPage) onIngestClick(ctx app.Context, e app.Event) {
	i.running = true
	i.result = ""
	i.error = ""
	

	i.runIngest(ctx)
}

// runIngest calls the API to trigger ingestion
func (i *IngestPage) runIngest(ctx app.Context) {
	ctx.Async(func() {
		res := app.Window().Call("fetch", "/api/ingest", map[string]interface{}{
			"method": "POST",
		})

		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) interface{} {
			if len(args) == 0 {
				return nil
			}
			response := args[0]

			status := response.Get("status").Int()

			response.Call("text").Call("then", app.FuncOf(func(this app.Value, args []app.Value) interface{} {
				if len(args) == 0 {
					return nil
				}

				text := args[0].String()

				ctx.Dispatch(func(ctx app.Context) {
					i.running = false
					if status >= 200 && status < 300 {
						i.result = "Ingestion completed successfully! " + text
					} else {
						i.error = "Ingestion failed: " + text
					}
				})

				return nil
			}))

			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) interface{} {
			ctx.Dispatch(func(ctx app.Context) {
				i.running = false
				i.error = "Network error: Could not connect to server"
			})
			return nil
		}))
	})
}
