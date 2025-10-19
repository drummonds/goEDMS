//go:build js && wasm
// +build js,wasm

package main

import (
	"github.com/drummonds/goEDMS/webapp"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

func main() {
	// Register routes for the client-side app
	app.Route("/app", func() app.Composer { return &webapp.HomePage{} })
	app.Route("/app/", func() app.Composer { return &webapp.HomePage{} })
	app.Route("/app/browse", func() app.Composer { return &webapp.BrowsePage{} })
	app.Route("/app/search", func() app.Composer { return &webapp.SearchPage{} })

	// This main function is for the WASM build only
	// It initializes the go-app when running in the browser
	app.RunWhenOnBrowser()
}
