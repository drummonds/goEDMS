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
			app.Main().Body(
				app.Div().Class("content").Body(
					a.renderPage(),
				),
			),
		)
}

// renderPage renders the current page based on the route
func (a *App) renderPage() app.UI {
	switch app.Window().URL().Path {
	case "/app", "/app/":
		return &HomePage{}
	case "/app/search":
		return &SearchPage{}
	case "/app/browse":
		return &BrowsePage{}
	default:
		return &HomePage{}
	}
}
