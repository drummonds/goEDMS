package webapp

import (
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// NavBar is the navigation bar component
type NavBar struct {
	app.Compo
}

// Render renders the navigation bar
func (n *NavBar) Render() app.UI {
	return app.Nav().
		Class("navbar").
		Body(
			app.Div().Class("navbar-brand").Body(
				app.H1().Text("goEDMS"),
			),
			app.Div().Class("navbar-menu").Body(
				app.A().
					Href("/app").
					Class("navbar-item").
					Body(app.Text("Home")),
				app.A().
					Href("/app/browse").
					Class("navbar-item").
					Body(app.Text("Browse")),
				app.A().
					Href("/app/search").
					Class("navbar-item").
					Body(app.Text("Search")),
			),
		)
}
