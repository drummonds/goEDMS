package webapp

import (
	"net/http"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// Handler returns an HTTP handler for the web app
func Handler() http.Handler {
	// Configure the app - all routes use the App component which includes navbar/sidebar
	app.Route("/", func() app.Composer { return &App{} })
	app.Route("/browse", func() app.Composer { return &App{} })
	app.Route("/ingest", func() app.Composer { return &App{} })
	app.Route("/clean", func() app.Composer { return &App{} })
	app.Route("/search", func() app.Composer { return &App{} })
	app.Route("/wordcloud", func() app.Composer { return &App{} })
	app.Route("/jobs", func() app.Composer { return &App{} })
	app.Route("/about", func() app.Composer { return &App{} })
	app.Route("/manuallog", func() app.Composer { return &App{} })
	app.Route("/edit/", func() app.Composer { return &EditPage{} })
	app.RouteWithRegexp("^/edit/.+$", func() app.Composer { return &EditPage{} })
	app.RunWhenOnBrowser()

	// Create and return the handler
	// wasm_exec.js is served at /wasm_exec.js by Echo (from public/built)
	// app.wasm is served from /web/app.wasm by Echo
	return &app.Handler{
		Name:        "godocs",
		Title:       "godocs",
		Description: "Electronic Document Management System",
		Icon: app.Icon{
			Default: "/favicon.ico",
		},
		Styles: []string{
			"/webapp/webapp.css",
			"/webapp/wordcloud.css",
		},
		Scripts: []string{
			"/config.js", // Load backend API configuration
		},
		RawHeaders: []string{
			`<meta name="viewport" content="width=device-width, initial-scale=1">`,
		},
	}
}
