package webapp

import (
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// NotFoundPage displays a 404 error message
type NotFoundPage struct {
	app.Compo
}

// OnNav is called when this page is navigated to
func (p *NotFoundPage) OnNav(ctx app.Context) {
	path := ctx.Page().URL().Path
	url := ctx.Page().URL().String()

	// Log to console immediately (always works)
	if app.IsClient {
		app.Window().Get("console").Call("warn",
			"[404 Page Not Found]",
			"path:", path,
			"url:", url)
	}

	// Try to log 404 error to backend
	// This may fail if ServiceWorker intercepts it
	LogWarn(ctx, "404 Page Not Found", map[string]interface{}{
		"component": "NotFoundPage",
		"path":      path,
		"url":       url,
		"referrer":  ctx.Page().URL().Query().Get("referrer"),
	})
}

// Render renders the 404 page
func (p *NotFoundPage) Render() app.UI {
	return app.Div().
		Class("not-found-page").
		Body(
			app.Div().
				Class("not-found-container").
				Body(
					app.H1().
						Class("not-found-title").
						Text("404"),
					app.H2().
						Class("not-found-subtitle").
						Text("Page Not Found"),
					app.P().
						Class("not-found-message").
						Text("The page you're looking for doesn't exist or has been moved."),
					app.Div().
						Class("not-found-actions").
						Body(
							app.A().
								Href("/").
								Class("not-found-home-link").
								Text("🏠 Go to Home Page"),
						),
				),
		)
}
