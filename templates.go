package main

import (
	"bytes"
	"embed"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/drummonds/godocs/config"
	"github.com/drummonds/godocs/database"
	"github.com/drummonds/godocs/engine"
	"github.com/drummonds/godocs/internal/build"
	"github.com/drummonds/lofigui"
	"github.com/flosch/pongo2/v6"
	"github.com/labstack/echo/v4"
)

//go:embed templates/*
var templatesFS embed.FS

// embedLoader implements pongo2.TemplateLoader for embed.FS
type embedLoader struct {
	fs     embed.FS
	prefix string // e.g. "templates"
}

func (l *embedLoader) Abs(base, name string) string {
	if filepath.IsAbs(name) || base == "" {
		return name
	}
	return filepath.Join(filepath.Dir(base), name)
}

func (l *embedLoader) Get(path string) (io.Reader, error) {
	// Try with prefix first, then without
	data, err := l.fs.ReadFile(filepath.Join(l.prefix, path))
	if err != nil {
		data, err = l.fs.ReadFile(path)
		if err != nil {
			return nil, err
		}
	}
	return bytes.NewReader(data), nil
}

// TemplateRenderer holds the pongo2 template set and app dependencies
type TemplateRenderer struct {
	templateSet *pongo2.TemplateSet
	app         *lofigui.App
	db          database.Repository
	config      config.ServerConfig
	engine      *engine.ServerHandler
}

// NewTemplateRenderer creates a renderer with embedded templates
func NewTemplateRenderer(app *lofigui.App, db database.Repository, cfg config.ServerConfig, eng *engine.ServerHandler) *TemplateRenderer {
	loader := &embedLoader{fs: templatesFS, prefix: "templates"}
	tplSet := pongo2.NewSet("embedded", loader)

	return &TemplateRenderer{
		templateSet: tplSet,
		app:         app,
		db:          db,
		config:      cfg,
		engine:      eng,
	}
}

// Render renders a template with merged lofigui state and page-specific data
func (tr *TemplateRenderer) Render(c echo.Context, name string, extra pongo2.Context) error {
	tpl, err := tr.templateSet.FromCache(name)
	if err != nil {
		return err
	}

	ctx := tr.buildContext(c, extra)
	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response().WriteHeader(http.StatusOK)
	return tpl.ExecuteWriter(ctx, c.Response().Writer)
}

// RenderWithStatus renders a template with a specific HTTP status code
func (tr *TemplateRenderer) RenderWithStatus(c echo.Context, name string, status int, extra pongo2.Context) error {
	tpl, err := tr.templateSet.FromCache(name)
	if err != nil {
		return err
	}

	ctx := tr.buildContext(c, extra)
	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response().WriteHeader(status)
	return tpl.ExecuteWriter(ctx, c.Response().Writer)
}

// buildContext creates the template context with app state and page data.
// We build the context manually rather than calling app.StateDict() to avoid
// a reentrant mutex deadlock in lofigui (StateDict holds Lock, then calls
// ControllerName which tries RLock).
func (tr *TemplateRenderer) buildContext(c echo.Context, extra pongo2.Context) pongo2.Context {
	ctx := pongo2.Context{
		"request":     c.Request(),
		"version":     tr.app.Version,
		"app_version": build.GetVersion(),
		"active_page": getActivePage(c.Request().URL.Path),
		"refresh":     "", // no polling for page renders
	}

	// Check if an action is running (for future polling support)
	if tr.app.IsActionRunning() {
		ctx["polling"] = "Running"
	} else {
		ctx["polling"] = "Stopped"
	}

	// Merge page-specific data
	if extra != nil {
		ctx.Update(extra)
	}

	return ctx
}

// getActivePage returns the active page identifier for navbar highlighting
func getActivePage(path string) string {
	switch {
	case path == "/":
		return "home"
	case path == "/search":
		return "search"
	case path == "/about":
		return "about"
	case path == "/tags" || strings.HasPrefix(path, "/tags/"):
		return "tags"
	case path == "/stories" || strings.HasPrefix(path, "/stories/"):
		return "stories"
	case path == "/jobs" || strings.HasPrefix(path, "/jobs/"):
		return "jobs"
	default:
		if len(path) > 10 && path[:10] == "/document/" {
			return "document"
		}
		return ""
	}
}
