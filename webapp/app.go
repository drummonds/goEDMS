package webapp

import (
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// RouteConfig defines a route and its render function
type RouteConfig struct {
	Path      string
	Render    func() app.UI
	MatchFunc func(path string) bool // Optional: custom path matching logic
}

// AppRoutes defines all application routes and their corresponding pages
var AppRoutes = []RouteConfig{
	{Path: "/", Render: func() app.UI { return &HomePage{} }},
	{Path: "/browse", Render: func() app.UI { return &BrowsePage{} }},
	{Path: "/ingest", Render: func() app.UI { return &IngestPage{} }},
	{Path: "/clean", Render: func() app.UI { return &CleanPage{} }},
	{Path: "/search", Render: func() app.UI { return &SearchPage{} }},
	{Path: "/wordcloud", Render: func() app.UI { return &WordCloudPage{} }},
	{Path: "/jobs", Render: func() app.UI { return &JobsPage{} }},
	{Path: "/about", Render: func() app.UI { return &AboutPage{} }},
	{Path: "/manuallog", Render: func() app.UI { return &ManualLogPage{} }},
	{
		Path:   "/edit/",
		Render: func() app.UI { return &EditPage{} },
		MatchFunc: func(path string) bool {
			return len(path) > 6 && path[:6] == "/edit/"
		},
	},
}

// RegisterRoutes registers all application routes
func RegisterRoutes() {
	for _, route := range AppRoutes {
		app.Route(route.Path, func() app.Composer { return &App{} })
		// For routes with custom match functions (like /edit/), also register regex
		if route.MatchFunc != nil {
			// Register a regex pattern that matches the path prefix followed by any content
			app.RouteWithRegexp("^"+route.Path+".+$", func() app.Composer { return &App{} })
		}
	}
}

// App is the root component of the application
type App struct {
	app.Compo
}

// Render renders the app
func (a *App) Render() app.UI {
	return app.Div().
		Class("app-container").
		Body(
			app.Header().Body(
				&NavBar{},
			),
			app.Div().Class("app-layout").Body(
				&Sidebar{},
				app.Main().Class("main-content").Body(
					app.Div().Class("content").Body(
						a.renderPage(),
					),
				),
			),
		)
}

// renderPage renders the current page based on the route
func (a *App) renderPage() app.UI {
	path := app.Window().URL().Path

	// Find matching route in AppRoutes
	for _, route := range AppRoutes {
		// Use custom match function if provided, otherwise exact match
		if route.MatchFunc != nil {
			if route.MatchFunc(path) {
				return route.Render()
			}
		} else if route.Path == path {
			return route.Render()
		}
	}

	// Default to NotFound page
	return &NotFoundPage{}
}
