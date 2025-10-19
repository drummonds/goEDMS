package webapp

import (
	"net/http"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// Handler returns an HTTP handler for the web app
func Handler() http.Handler {
	// Configure the app - routes should be relative to /app
	app.Route("/app", func() app.Composer { return &HomePage{} })
	app.Route("/app/", func() app.Composer { return &HomePage{} })
	app.Route("/app/browse", func() app.Composer { return &BrowsePage{} })
	app.Route("/app/search", func() app.Composer { return &SearchPage{} })
	app.RunWhenOnBrowser()

	// Create and return the handler
	// wasm_exec.js is served at /wasm_exec.js by Echo (from public/built)
	// app.wasm is served from /web/app.wasm by Echo
	return &app.Handler{
		Name:        "goEDMS",
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
