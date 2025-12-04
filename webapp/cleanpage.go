package webapp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// CleanPage allows users to clean the database by removing orphaned entries
type CleanPage struct {
	app.Compo
	running      bool
	result       string
	error        string
	deletedCount int
	scannedCount int
	movedCount   int
	jobID        string
	progress     int
	currentStep  string
	pollTicker   *time.Ticker
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
			app.P().Text("It will also find documents in storage that are not in the database and move them to the ingress folder for reprocessing (including any .yaml metadata and .txt OCR files)."),

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

// OnDismount stops the poll ticker when leaving the page
func (c *CleanPage) OnDismount() {
	if c.pollTicker != nil {
		c.pollTicker.Stop()
	}
}

// renderStatus renders the status section
func (c *CleanPage) renderStatus() app.UI {
	if c.running {
		statusText := "Starting cleanup..."
		if c.currentStep != "" {
			statusText = c.currentStep
		}

		return app.Div().Class("job-progress-container").Body(
			app.Div().Class("progress-bar").Body(
				app.Div().
					Class("progress-fill").
					Style("width", fmt.Sprintf("%d%%", c.progress)),
			),
			app.Div().Class("progress-text").Body(
				app.Text(fmt.Sprintf("%d%% - %s", c.progress, statusText)),
			),
		)
	}

	if c.error != "" {
		return app.Div().Class("error").Body(
			app.Text("Error: " + c.error),
		)
	}

	if c.result != "" {
		resultMsg := c.result
		details := []string{}

		if c.deletedCount > 0 {
			details = append(details, fmt.Sprintf("Removed %d orphaned database entries", c.deletedCount))
		}
		if c.movedCount > 0 {
			details = append(details, fmt.Sprintf("Moved %d orphaned documents to ingress", c.movedCount))
		}

		if len(details) > 0 {
			resultMsg = fmt.Sprintf("%s - %s.", c.result, joinStrings(details, ", "))
		} else {
			resultMsg = c.result + " - No issues found. Database is clean!"
		}

		return app.Div().Class("success").Body(
			app.P().Text(resultMsg),
			app.P().Text(fmt.Sprintf("Scanned: %d documents", c.scannedCount)),
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
	c.movedCount = 0

	c.runClean(ctx)
}

// runClean calls the API to trigger database cleaning
func (c *CleanPage) runClean(ctx app.Context) {
	ctx.Async(func() {
		res := app.Window().Call("fetch", BuildAPIURL("/api/clean"), map[string]interface{}{
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
					if status >= 200 && status < 300 {
						// Get job ID and start polling
						if jsonData.Truthy() {
							if jobID := jsonData.Get("jobId"); jobID.Truthy() {
								c.jobID = jobID.String()
								c.startPolling(ctx)
							} else {
								// No job ID - cleanup completed synchronously
								c.running = false
								c.result = "Cleanup completed successfully!"
							}
						}
					} else {
						c.running = false
						c.error = fmt.Sprintf("Cleanup failed with status: %d", status)
						LogError(ctx, "Cleanup job failed", map[string]interface{}{
							"component": "CleanPage",
							"action":    "runCleanup",
							"status":    status,
						})
					}
				})

				return nil
			}))

			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) interface{} {
			ctx.Dispatch(func(ctx app.Context) {
				c.running = false
				c.error = "Network error: Could not connect to server"
				LogError(ctx, "Network error during cleanup", map[string]interface{}{
					"component": "CleanPage",
					"action":    "runCleanup",
				})
			})
			return nil
		}))
	})
}

// startPolling starts polling for job status
func (c *CleanPage) startPolling(ctx app.Context) {
	c.pollTicker = time.NewTicker(1 * time.Second)

	ctx.Async(func() {
		for range c.pollTicker.C {
			c.pollJobStatus(ctx)
		}
	})
}

// pollJobStatus fetches the current job status
func (c *CleanPage) pollJobStatus(ctx app.Context) {
	url := BuildAPIURL("/api/jobs/" + c.jobID)
	res := app.Window().Call("fetch", url)

	res.Call("then", app.FuncOf(func(this app.Value, args []app.Value) interface{} {
		if len(args) == 0 {
			return nil
		}
		response := args[0]

		response.Call("json").Call("then", app.FuncOf(func(this app.Value, args []app.Value) interface{} {
			if len(args) == 0 {
				return nil
			}

			jsonData := args[0]

			ctx.Dispatch(func(ctx app.Context) {
				if !jsonData.Truthy() {
					return
				}

				// Parse job data
				jsonStr := app.Window().Get("JSON").Call("stringify", jsonData).String()
				var job Job
				if err := json.Unmarshal([]byte(jsonStr), &job); err != nil {
					return
				}

				c.progress = job.Progress
				c.currentStep = job.CurrentStep

				// Check if job is complete
				if job.Status == "completed" || job.Status == "failed" {
					c.pollTicker.Stop()
					c.running = false

					if job.Status == "completed" {
						c.result = "Cleanup completed successfully!"
						// Parse result for counts
						if job.Result != "" {
							var result map[string]interface{}
							if err := json.Unmarshal([]byte(job.Result), &result); err == nil {
								if v, ok := result["deleted"].(float64); ok {
									c.deletedCount = int(v)
								}
								if v, ok := result["scanned"].(float64); ok {
									c.scannedCount = int(v)
								}
								if v, ok := result["moved"].(float64); ok {
									c.movedCount = int(v)
								}
							}
						}
					} else {
						c.error = "Cleanup failed: " + job.Error
					}
				}
			})

			return nil
		}))

		return nil
	}))
}

// joinStrings joins a slice of strings with a separator
func joinStrings(strs []string, sep string) string {
	return strings.Join(strs, sep)
}
