//go:build js && wasm
// +build js,wasm

package main

import (
	"github.com/drummonds/goEDMS/webapp"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

func main() {
	// Register routes for the client-side app - all use App component with navbar/sidebar
	app.Route("/", func() app.Composer { return &webapp.App{} })
	app.Route("/browse", func() app.Composer { return &webapp.App{} })
	app.Route("/ingest", func() app.Composer { return &webapp.App{} })
	app.Route("/clean", func() app.Composer { return &webapp.App{} })
	app.Route("/search", func() app.Composer { return &webapp.App{} })
	app.Route("/about", func() app.Composer { return &webapp.App{} })

	// This main function is for the WASM build only
	// It initializes the go-app when running in the browser
	app.RunWhenOnBrowser()
}
