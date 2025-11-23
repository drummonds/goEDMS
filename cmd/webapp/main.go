//go:build js && wasm
// +build js,wasm

package main

import (
	"github.com/drummonds/godocs/webapp"
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

func main() {
	// Register all routes from shared configuration
	webapp.RegisterRoutes()

	// This main function is for the WASM build only
	// It initializes the go-app when running in the browser
	app.RunWhenOnBrowser()
}
