package webapp

import (
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

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

	// Handle edit pages with dynamic ULID
	if len(path) > 6 && path[:6] == "/edit/" {
		return &EditPage{}
	}

	switch path {
	case "/":
		return &HomePage{}
	case "/browse":
		return &BrowsePage{}
	case "/ingest":
		return &IngestPage{}
	case "/clean":
		return &CleanPage{}
	case "/search":
		return &SearchPage{}
	case "/wordcloud":
		return &WordCloudPage{}
	case "/jobs":
		return &JobsPage{}
	case "/about":
		return &AboutPage{}
	case "/manuallog":
		return &ManualLogPage{}
	default:
		return &NotFoundPage{}
	}
}
