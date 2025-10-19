package webapp

import (
	"fmt"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// CleanPage allows users to clean the database by removing orphaned entries
type CleanPage struct {
	app.Compo
	running       bool
	result        string
	error         string
	deletedCount  int
	scannedCount  int
}

// Render renders the clean page
func (c *CleanPage) Render() app.UI {
	buttonText := "Clean Database Now"
	if c.running {
		buttonText = "Scanning..."
	}

	return app.Div().
		Class("clean-page").
		Body(
			app.H2().Text("Database Cleanup"),
			app.P().Text("This tool will scan all documents in the database and verify that their files still exist on disk. Any database entries for missing files will be removed."),

			app.Div().Class("warning").Body(
				app.P().Text("⚠️ Warning: This operation will permanently delete database entries for missing files. Make sure you have a backup if needed."),
			),

			app.Div().Class("clean-controls").Body(
				app.Button().
					Class("btn-danger").
					Disabled(c.running).
					OnClick(c.onCleanClick).
					Body(app.Text(buttonText)),
			),

			c.renderStatus(),
		)
}

// renderStatus renders the status section
func (c *CleanPage) renderStatus() app.UI {
	if c.running {
		statusText := "Scanning documents and checking files..."
		if c.scannedCount > 0 {
			statusText = fmt.Sprintf("Scanned: %d documents", c.scannedCount)
		}
		return app.Div().Class("loading").Body(
			app.Text(statusText),
		)
	}

	if c.error != "" {
		return app.Div().Class("error").Body(
			app.Text("Error: "+c.error),
		)
	}

	if c.result != "" {
		resultMsg := c.result
		if c.deletedCount > 0 {
			resultMsg = fmt.Sprintf("%s - Removed %d orphaned database entries.", c.result, c.deletedCount)
		} else {
			resultMsg = c.result + " - No orphaned entries found. Database is clean!"
		}
		return app.Div().Class("success").Body(
			app.P().Text(resultMsg),
		)
	}

	return app.Div()
}

// onCleanClick handles the clean button click
func (c *CleanPage) onCleanClick(ctx app.Context, e app.Event) {
	c.running = true
	c.result = ""
	c.error = ""
	c.deletedCount = 0
	c.scannedCount = 0
	

	c.runClean(ctx)
}

// runClean calls the API to trigger database cleaning
func (c *CleanPage) runClean(ctx app.Context) {
	ctx.Async(func() {
		res := app.Window().Call("fetch", "/api/clean", map[string]interface{}{
			"method": "POST",
		})

		res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) interface{} {
			if len(args) == 0 {
				return nil
			}
			response := args[0]

			status := response.Get("status").Int()

			response.Call("json").Call("then", app.FuncOf(func(this app.Value, args []app.Value) interface{} {
				if len(args) == 0 {
					return nil
				}

				jsonData := args[0]

				ctx.Dispatch(func(ctx app.Context) {
					c.running = false
					if status >= 200 && status < 300 {
						// Try to extract counts from response
						if jsonData.Truthy() {
							if deleted := jsonData.Get("deleted"); deleted.Truthy() {
								c.deletedCount = deleted.Int()
							}
							if scanned := jsonData.Get("scanned"); scanned.Truthy() {
								c.scannedCount = scanned.Int()
							}
							if msg := jsonData.Get("message"); msg.Truthy() {
								c.result = msg.String()
							} else {
								c.result = "Cleanup completed successfully!"
							}
						} else {
							c.result = "Cleanup completed successfully!"
						}
					} else {
						c.error = fmt.Sprintf("Cleanup failed with status: %d", status)
					}
				})

				return nil
			}))

			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) interface{} {
			ctx.Dispatch(func(ctx app.Context) {
				c.running = false
				c.error = "Network error: Could not connect to server"
			})
			return nil
		}))
	})
}
