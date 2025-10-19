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
				app.H1().Text("goEDMS"),
			),
			app.Div().Class("navbar-menu").Body(
				app.A().
					Href("/").
					Class("navbar-item").
					Body(app.Text("Home")),
				app.A().
					Href("/browse").
					Class("navbar-item").
					Body(app.Text("Browse")),
				app.A().
					Href("/ingest").
					Class("navbar-item").
					Body(app.Text("Ingest")),
				app.A().
					Href("/clean").
					Class("navbar-item").
					Body(app.Text("Clean")),
				app.A().
					Href("/search").
					Class("navbar-item").
					Body(app.Text("Search")),
			),
		)
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
