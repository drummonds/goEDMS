package webapp

import (
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// Sidebar is the left sidebar menu component
type Sidebar struct {
	app.Compo
	isOpen bool
}

// OnMount is called when the component is mounted
func (s *Sidebar) OnMount(ctx app.Context) {
	s.isOpen = s.getSidebarState(ctx)
}

// OnNav is called when navigation occurs
func (s *Sidebar) OnNav(ctx app.Context) {
	s.isOpen = s.getSidebarState(ctx)
}

// Render renders the sidebar
func (s *Sidebar) Render() app.UI {
	class := "sidebar"
	if s.isOpen {
		class += " sidebar-open"
	}

	return app.Aside().
		Class(class).
		Body(
			app.Div().Class("sidebar-header").Body(
				app.H2().Text("Menu"),
			),
			app.Nav().Class("sidebar-nav").Body(
				s.renderNavItem("🏠", "Home", "/"),
				s.renderNavItem("📁", "Browse Documents", "/browse"),
				s.renderNavItem("📥", "Ingest Now", "/ingest"),
				s.renderNavItem("🧹", "Clean Database", "/clean"),
				s.renderNavItem("🔍", "Search", "/search"),
				s.renderNavItem("📊", "Word Cloud", "/wordcloud"),
				s.renderNavItem("ℹ️", "About", "/about"),
			),
		)
}

// renderNavItem creates a navigation item
func (s *Sidebar) renderNavItem(icon, label, href string) app.UI {
	currentPath := app.Window().URL().Path
	class := "sidebar-item"
	if currentPath == href {
		class += " sidebar-item-active"
	}

	return app.A().
		Href(href).
		Class(class).
		Body(
			app.Span().Class("sidebar-icon").Text(icon),
			app.Span().Class("sidebar-label").Text(label),
		)
}

// getSidebarState retrieves the sidebar open/closed state from local storage
func (s *Sidebar) getSidebarState(ctx app.Context) bool {
	var isOpen bool
	ctx.LocalStorage().Get("sidebar-open", &isOpen)
	return isOpen
}
