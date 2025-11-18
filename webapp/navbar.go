package webapp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// Version info - can be set at build time with -ldflags
var (
	Version   = "dev"
	BuildDate = ""
)

// NavBar is the navigation bar component
type NavBar struct {
	app.Compo
	activeJobCount int
	refreshTicker  *time.Ticker
}

// Render renders the navigation bar
func (n *NavBar) Render() app.UI {
	return app.Nav().
		Class("navbar").
		Body(
			app.Button().
				Class("hamburger-menu").
				ID("menu-toggle").
				OnClick(n.onMenuToggle).
				Body(
					// Three horizontal lines for hamburger menu
					app.Span().Class("hamburger-line"),
					app.Span().Class("hamburger-line"),
					app.Span().Class("hamburger-line"),
				),
			app.Div().Class("navbar-brand").Body(
				app.H1().Text("godocs"),
				app.Span().Class("version-info").Body(
					app.Text(n.getVersionInfo()),
				),
			),
			app.Div().Class("navbar-menu").Body(
				app.Button().
					Class("navbar-item").
					OnClick(n.navigateTo("/")).
					Body(app.Text("Home")),
				app.Button().
					Class("navbar-item").
					OnClick(n.navigateTo("/browse")).
					Body(app.Text("Browse")),
				app.Button().
					Class("navbar-item").
					OnClick(n.navigateTo("/ingest")).
					Body(app.Text("Ingest")),
				app.Button().
					Class("navbar-item").
					OnClick(n.navigateTo("/clean")).
					Body(app.Text("Clean")),
				app.Button().
					Class("navbar-item").
					OnClick(n.navigateTo("/search")).
					Body(app.Text("Search")),
				app.Button().
					Class("navbar-item").
					OnClick(n.navigateTo("/jobs")).
					Body(app.Text("Jobs")),
			),
		)
}

// navigateTo creates a navigation handler for the given path
func (n *NavBar) navigateTo(path string) func(ctx app.Context, e app.Event) {
	return func(ctx app.Context, e app.Event) {
		e.PreventDefault()
		ctx.Navigate(path)
	}
}

// onMenuToggle handles the hamburger menu click
func (n *NavBar) onMenuToggle(ctx app.Context, e app.Event) {
	// Dispatch a custom event to toggle the sidebar
	ctx.Dispatch(func(ctx app.Context) {
		ctx.LocalStorage().Set("sidebar-open", !n.isSidebarOpen(ctx))
		ctx.Reload()
	})
}

// isSidebarOpen checks if the sidebar is currently open
func (n *NavBar) isSidebarOpen(ctx app.Context) bool {
	var isOpen bool
	ctx.LocalStorage().Get("sidebar-open", &isOpen)
	return isOpen
}

// OnMount is called when the component is mounted
func (n *NavBar) OnMount(ctx app.Context) {
	n.loadActiveJobCount(ctx)

	// Start auto-refresh every 5 seconds
	ctx.Async(func() {
		n.refreshTicker = time.NewTicker(5 * time.Second)
		for range n.refreshTicker.C {
			n.loadActiveJobCount(ctx)
		}
	})
}

// OnDismount is called when the component is unmounted
func (n *NavBar) OnDismount() {
	if n.refreshTicker != nil {
		n.refreshTicker.Stop()
	}
}

// getVersionInfo returns formatted version and date information with job count
func (n *NavBar) getVersionInfo() string {
	date := BuildDate
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	jobInfo := ""
	if n.activeJobCount > 0 {
		jobInfo = fmt.Sprintf(" | %d active job", n.activeJobCount)
		if n.activeJobCount > 1 {
			jobInfo += "s"
		}
	}

	return fmt.Sprintf("%s | %s%s", Version, date, jobInfo)
}

// loadActiveJobCount fetches the count of active jobs from the API
func (n *NavBar) loadActiveJobCount(ctx app.Context) {
	ctx.Async(func() {
		res := app.Window().Call("fetch", BuildAPIURL("/api/jobs/active"))

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
						// Parse jobs array to count them
						if jsonData.Truthy() && jsonData.Type() != app.TypeNull {
							var jobs []Job
							jsonStr := app.Window().Get("JSON").Call("stringify", jsonData).String()
							if err := json.Unmarshal([]byte(jsonStr), &jobs); err == nil {
								n.activeJobCount = len(jobs)
							} else {
								n.activeJobCount = 0
							}
						} else {
							n.activeJobCount = 0
						}
					} else {
						n.activeJobCount = 0
					}
				})

				return nil
			}))

			return nil
		})).Call("catch", app.FuncOf(func(this app.Value, args []app.Value) interface{} {
			// Silently fail - don't update job count on network error
			return nil
		}))
	})
}
