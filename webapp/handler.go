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
	app.RunWhenOnBrowser()

	// Create and return the handler
	// wasm_exec.js is served at /wasm_exec.js by Echo (from public/built)
	// app.wasm is served from /web/app.wasm by Echo
	return &app.Handler{
		Name:        "goEDMS",
		Title:       "goEDMS",
		Description: "Electronic Document Management System",
		Icon: app.Icon{
			Default: "/favicon.ico",
		},
		Styles: []string{
			"/webapp/webapp.css",
		},
		RawHeaders: []string{
			`<meta name="viewport" content="width=device-width, initial-scale=1">`,
		},
	}
}
