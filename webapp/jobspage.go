package webapp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// JobsPage displays and manages background jobs
type JobsPage struct {
	app.Compo
	jobs          []Job
	loading       bool
	error         string
	autoRefresh   bool
	refreshTicker *time.Ticker
}

// OnMount is called when the component is mounted
func (j *JobsPage) OnMount(ctx app.Context) {
	j.autoRefresh = true
	j.loadJobs(ctx)

	// Start auto-refresh every 2 seconds
	ctx.Async(func() {
		j.refreshTicker = time.NewTicker(2 * time.Second)
		for range j.refreshTicker.C {
			if j.autoRefresh {
				j.loadJobs(ctx)
			}
		}
	})
}

// OnDismount is called when the component is unmounted
func (j *JobsPage) OnDismount() {
	if j.refreshTicker != nil {
		j.refreshTicker.Stop()
	}
}

// Render renders the jobs page
func (j *JobsPage) Render() app.UI {
	return app.Div().
		Class("jobs-page").
		Body(
			app.H2().Text("Background Jobs"),
			app.P().Text("View and monitor background jobs for document processing, cleanup, and other tasks."),

			app.Div().Class("jobs-controls").Body(
				app.Button().
					Class("btn-primary").
					OnClick(j.onRefreshClick).
					Disabled(j.loading).
					Body(app.Text("Refresh")),
				app.Label().Class("auto-refresh-label").Body(
					app.Input().
						Type("checkbox").
						Checked(j.autoRefresh).
						OnChange(j.onAutoRefreshChange),
					app.Text(" Auto-refresh"),
				),
			),

			j.renderStatus(),
		)
}

// renderStatus renders the jobs list or status messages
func (j *JobsPage) renderStatus() app.UI {
	if j.loading && len(j.jobs) == 0 {
		return app.Div().Class("loading").Body(
			app.Text("Loading jobs..."),
		)
	}

	if j.error != "" {
		return app.Div().Class("error").Body(
			app.Text("Error: " + j.error),
		)
	}

	if len(j.jobs) == 0 {
		return app.Div().Class("info").Body(
			app.P().Text("No jobs found. Jobs are created when you trigger ingestion, cleanup, or other background operations."),
		)
	}

	return app.Div().Class("jobs-list").Body(
		j.renderJobsList()...,
	)
}

// renderJobsList renders the list of jobs
func (j *JobsPage) renderJobsList() []app.UI {
	var items []app.UI

	for i := range j.jobs {
		job := &j.jobs[i]
		items = append(items, j.renderJob(job))
	}

	return items
}

// renderJob renders a single job card
func (j *JobsPage) renderJob(job *Job) app.UI {
	statusClass := "job-card job-" + job.Status

	return app.Div().
		Class(statusClass).
		Body(
			app.Div().Class("job-header").Body(
				app.Div().Class("job-type").Body(
					app.Strong().Text(j.formatJobType(job.Type)),
					app.Span().Class("job-status-badge job-status-"+job.Status).
						Body(app.Text(job.Status)),
				),
				app.Div().Class("job-time").Body(
					app.Text(j.formatTime(job.CreatedAt)),
				),
			),

			app.If(job.Status == "running",
				func() app.UI {
					return app.Div().Class("job-progress").Body(
						app.Div().Class("progress-bar").Body(
							app.Div().
								Class("progress-fill").
								Style("width", fmt.Sprintf("%d%%", job.Progress)),
						),
						app.Div().Class("progress-text").Body(
							app.Text(fmt.Sprintf("%d%% - %s", job.Progress, job.CurrentStep)),
						),
					)
				},
			),

			app.If(job.Message != "",
				func() app.UI {
					return app.Div().Class("job-message").Body(
						app.Text(job.Message),
					)
				},
			),

			app.If(job.Error != "",
				func() app.UI {
					return app.Div().Class("job-error").Body(
						app.Strong().Text("Error: "),
						app.Text(job.Error),
					)
				},
			),

			app.If(job.Result != "",
				func() app.UI {
					return app.Div().Class("job-result").Body(
						app.Text(j.formatResult(job.Result)),
					)
				},
			),

			app.Div().Class("job-footer").Body(
				app.Div().Class("job-id").Body(
					app.Text("ID: " + job.ID),
				),
				app.If(job.CompletedAt != "",
					func() app.UI {
						return app.Div().Class("job-completed").Body(
							app.Text("Completed: " + j.formatTime(job.CompletedAt)),
						)
					},
				),
			),
		)
}

// formatJobType converts job type to readable format
func (j *JobsPage) formatJobType(jobType string) string {
	switch jobType {
	case "ingestion":
		return "Document Ingestion"
	case "cleanup":
		return "Database Cleanup"
	case "wordcloud":
		return "Word Cloud Recalculation"
	case "search_reindex":
		return "Search Reindex"
	default:
		return strings.Title(jobType)
	}
}

// formatTime formats ISO time string to readable format
func (j *JobsPage) formatTime(timeStr string) string {
	if timeStr == "" {
		return ""
	}

	// Try to parse ISO 8601 format
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		// Try without nanoseconds
		t, err = time.Parse("2006-01-02T15:04:05Z", timeStr)
		if err != nil {
			return timeStr
		}
	}

	// Format as relative time if recent
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "Just now"
	} else if diff < time.Hour {
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}

	return t.Format("Jan 2, 2006 at 3:04 PM")
}

// formatResult formats JSON result string
func (j *JobsPage) formatResult(result string) string {
	// Try to parse as JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return result
	}

	// Format nicely
	var parts []string
	if val, ok := data["filesProcessed"]; ok {
		parts = append(parts, fmt.Sprintf("Processed: %.0f files", val))
	}
	if val, ok := data["filesTotal"]; ok {
		parts = append(parts, fmt.Sprintf("Total: %.0f", val))
	}
	if val, ok := data["duplicates"]; ok && val.(float64) > 0 {
		parts = append(parts, fmt.Sprintf("Duplicates: %.0f", val))
	}
	if val, ok := data["errors"]; ok && val.(float64) > 0 {
		parts = append(parts, fmt.Sprintf("Errors: %.0f", val))
	}
	if val, ok := data["scanned"]; ok {
		parts = append(parts, fmt.Sprintf("Scanned: %.0f", val))
	}
	if val, ok := data["deleted"]; ok && val.(float64) > 0 {
		parts = append(parts, fmt.Sprintf("Deleted: %.0f", val))
	}
	if val, ok := data["moved"]; ok && val.(float64) > 0 {
		parts = append(parts, fmt.Sprintf("Moved: %.0f", val))
	}

	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}

	return result
}

// onRefreshClick handles the refresh button click
func (j *JobsPage) onRefreshClick(ctx app.Context, e app.Event) {
	j.loadJobs(ctx)
}

// onAutoRefreshChange handles auto-refresh checkbox change
func (j *JobsPage) onAutoRefreshChange(ctx app.Context, e app.Event) {
	j.autoRefresh = ctx.JSSrc().Get("checked").Bool()
	ctx.Update()
}

// loadJobs fetches jobs from the API
func (j *JobsPage) loadJobs(ctx app.Context) {
	j.loading = true
	j.error = ""
	ctx.Update()

	ctx.Async(func() {
		res := app.Window().Call("fetch", BuildAPIURL("/api/jobs?limit=50"))

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
					j.loading = false
					if status >= 200 && status < 300 {
						// Parse jobs array
						if jsonData.Truthy() && jsonData.Type() != app.TypeNull {
							var jobs []Job
							jsonStr := app.Window().Get("JSON").Call("stringify", jsonData).String()
							if err := json.Unmarshal([]byte(jsonStr), &jobs); err == nil {
								j.jobs = jobs
							} else {
								j.error = "Failed to parse jobs: " + err.Error()
								LogError(ctx, "Failed to parse jobs response", map[string]interface{}{
									"component": "JobsPage",
									"action":    "fetchJobs",
									"error":     err.Error(),
								})
							}
						} else {
							j.jobs = []Job{}
						}
					} else {
						j.error = fmt.Sprintf("Failed to load jobs (status: %d)", status)
						LogError(ctx, "Failed to load jobs", map[string]interface{}{
							"component": "JobsPage",
							"action":    "fetchJobs",
							"status":    status,
						})
					}
				})

				return nil
			}))

			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) interface{} {
			ctx.Dispatch(func(ctx app.Context) {
				j.loading = false
				j.error = "Network error: Could not connect to server"
				LogError(ctx, "Network error loading jobs", map[string]interface{}{
					"component": "JobsPage",
					"action":    "fetchJobs",
				})
			})
			return nil
		}))
	})
}
