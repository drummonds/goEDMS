package docs

import (
	"embed"
	"net/http"

	"github.com/labstack/echo/v4"
)

// DocsHandler handles all documentation and Swagger UI endpoints
type DocsHandler struct {
	DocsFS      embed.FS
	SwaggerUIFS embed.FS
}

// GetSwaggerJSON serves the swagger.json file
func (h *DocsHandler) GetSwaggerJSON(c echo.Context) error {
	data, err := h.DocsFS.ReadFile("docs/swagger.json")
	if err != nil {
		return c.String(http.StatusNotFound, "swagger.json not found")
	}
	return c.Blob(http.StatusOK, "application/json", data)
}

// GetOpenAPIYAML serves the openapi.yaml file
func (h *DocsHandler) GetOpenAPIYAML(c echo.Context) error {
	data, err := h.DocsFS.ReadFile("docs/openapi.yaml")
	if err != nil {
		return c.String(http.StatusNotFound, "openapi.yaml not found")
	}
	return c.Blob(http.StatusOK, "text/yaml", data)
}

// GetSwaggerUICSS serves the Swagger UI CSS file
func (h *DocsHandler) GetSwaggerUICSS(c echo.Context) error {
	data, err := h.SwaggerUIFS.ReadFile("static/swagger-ui/swagger-ui.css")
	if err != nil {
		return c.String(http.StatusNotFound, "swagger-ui.css not found")
	}
	return c.Blob(http.StatusOK, "text/css", data)
}

// GetSwaggerUIBundle serves the Swagger UI bundle JavaScript
func (h *DocsHandler) GetSwaggerUIBundle(c echo.Context) error {
	data, err := h.SwaggerUIFS.ReadFile("static/swagger-ui/swagger-ui-bundle.js")
	if err != nil {
		return c.String(http.StatusNotFound, "swagger-ui-bundle.js not found")
	}
	return c.Blob(http.StatusOK, "application/javascript", data)
}

// GetSwaggerUIPreset serves the Swagger UI standalone preset JavaScript
func (h *DocsHandler) GetSwaggerUIPreset(c echo.Context) error {
	data, err := h.SwaggerUIFS.ReadFile("static/swagger-ui/swagger-ui-standalone-preset.js")
	if err != nil {
		return c.String(http.StatusNotFound, "swagger-ui-standalone-preset.js not found")
	}
	return c.Blob(http.StatusOK, "application/javascript", data)
}

// GetSwaggerUI serves the Swagger UI HTML page
func (h *DocsHandler) GetSwaggerUI(c echo.Context) error {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>godocs API Documentation</title>
    <link rel="stylesheet" type="text/css" href="/api/docs/swagger-ui.css">
    <style>
        html { box-sizing: border-box; overflow-y: scroll; }
        *, *:before, *:after { box-sizing: inherit; }
        body { margin: 0; background: #fafafa; }
        .topbar { display: none; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="/api/docs/swagger-ui-bundle.js"></script>
    <script src="/api/docs/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            SwaggerUIBundle({
                url: "/api/docs/swagger.json",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                layout: "BaseLayout"
            });
        };
    </script>
</body>
</html>`
	return c.HTML(http.StatusOK, html)
}
