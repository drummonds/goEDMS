package main

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/drummonds/godocs/config"
	"github.com/drummonds/godocs/database"
	"github.com/drummonds/godocs/internal/build"
	"github.com/flosch/pongo2/v6"
	"github.com/labstack/echo/v4"
	"github.com/oklog/ulid/v2"
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

// TemplateRenderer holds the pongo2 template set and app dependencies.
// Callback functions decouple this from engine/lofigui so the same code
// compiles for both server and WASM targets.
type TemplateRenderer struct {
	templateSet       *pongo2.TemplateSet
	db                database.Repository
	config            config.ServerConfig
	version           string
	isActionRunning   func() bool
	checkThumbnail    func(string) bool
	writeTagAliases   func(string, database.Repository) error
	runCleanupAsync   func() (*database.Job, error)
	runIngestionAsync func() (*database.Job, error)
	cancelJob         func(ulid.ULID) error
}

// NewTemplateSet creates a pongo2 template set from embedded templates
func NewTemplateSet() *pongo2.TemplateSet {
	loader := &embedLoader{fs: templatesFS, prefix: "templates"}
	return pongo2.NewSet("embedded", loader)
}

// Render renders a template with merged lofigui state and page-specific data.
// Templates are rendered to a buffer first so errors produce proper HTTP 500
// responses instead of partial HTML with a 200 status.
func (tr *TemplateRenderer) Render(c echo.Context, name string, extra pongo2.Context) error {
	return tr.renderToResponse(c, name, http.StatusOK, extra)
}

// RenderWithStatus renders a template with a specific HTTP status code
func (tr *TemplateRenderer) RenderWithStatus(c echo.Context, name string, status int, extra pongo2.Context) error {
	return tr.renderToResponse(c, name, status, extra)
}

// renderToResponse renders a template to a buffer, then writes the response.
// If template execution fails, an error is returned before headers are sent,
// allowing the error handler to return a proper error page.
func (tr *TemplateRenderer) renderToResponse(c echo.Context, name string, status int, extra pongo2.Context) error {
	tpl, err := tr.templateSet.FromCache(name)
	if err != nil {
		Logger.Error("Template not found", "template", name, "error", err)
		return fmt.Errorf("template %q: %w", name, err)
	}

	ctx := tr.buildContext(c, extra)

	// Render to buffer first to catch template execution errors
	var buf bytes.Buffer
	if err := tpl.ExecuteWriter(ctx, &buf); err != nil {
		Logger.Error("Template execution failed",
			"template", name,
			"error", err,
			"path", c.Request().URL.Path,
		)
		return fmt.Errorf("template %q execution: %w", name, err)
	}

	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response().WriteHeader(status)
	_, err = buf.WriteTo(c.Response().Writer)
	return err
}

// buildContext creates the template context with app state and page data.
// We build the context manually rather than calling app.StateDict() to avoid
// a reentrant mutex deadlock in lofigui (StateDict holds Lock, then calls
// ControllerName which tries RLock).
func (tr *TemplateRenderer) buildContext(c echo.Context, extra pongo2.Context) pongo2.Context {
	ctx := pongo2.Context{
		"request":     c.Request(),
		"version":     tr.version,
		"app_version": build.GetVersion(),
		"active_page": getActivePage(c.Request().URL.Path),
		"refresh":     "", // no polling for page renders
	}

	// Check if an action is running (for future polling support)
	if tr.isActionRunning() {
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
