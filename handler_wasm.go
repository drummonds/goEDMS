//go:build js && wasm

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall/js"

	"github.com/labstack/echo/v4"
)

// handleHTTPRequest returns a JS function that processes HTTP requests through Echo.
// JS signature: handleRequest(method, path, body) → Promise<{status, headers, body, isBytes, bodyBytes}>
func handleHTTPRequest(e *echo.Echo) func(this js.Value, args []js.Value) interface{} {
	return func(this js.Value, args []js.Value) interface{} {
		if len(args) < 2 {
			return js.ValueOf(map[string]interface{}{
				"status": 400,
				"body":   "missing method or path",
			})
		}

		method := args[0].String()
		path := args[1].String()
		body := ""
		if len(args) > 2 && !args[2].IsUndefined() && !args[2].IsNull() {
			body = args[2].String()
		}
		contentType := ""
		if len(args) > 3 && !args[3].IsUndefined() && !args[3].IsNull() {
			contentType = args[3].String()
		}

		// Create the request
		var req *http.Request
		if body != "" {
			req = httptest.NewRequest(method, path, strings.NewReader(body))
			if contentType != "" {
				req.Header.Set("Content-Type", contentType)
			} else {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
		} else {
			req = httptest.NewRequest(method, path, nil)
		}

		// Record the response
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		result := rec.Result()
		respBody := rec.Body.Bytes()

		// Build headers map
		headers := make(map[string]interface{})
		for k, v := range result.Header {
			if len(v) > 0 {
				headers[strings.ToLower(k)] = v[0]
			}
		}

		ct := result.Header.Get("Content-Type")
		isBinary := !strings.HasPrefix(ct, "text/") &&
			!strings.HasPrefix(ct, "application/json") &&
			ct != ""

		response := map[string]interface{}{
			"status":  result.StatusCode,
			"headers": headers,
			"isBytes": isBinary,
		}

		if isBinary {
			// Return binary data as Uint8Array
			dst := js.Global().Get("Uint8Array").New(len(respBody))
			js.CopyBytesToJS(dst, respBody)
			response["bodyBytes"] = dst
			response["body"] = ""
		} else {
			response["body"] = string(respBody)
			response["bodyBytes"] = js.Null()
		}

		return js.ValueOf(response)
	}
}

// createWASMErrorHandler returns a custom error handler for WASM mode
func createWASMErrorHandler(tr *TemplateRenderer) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		code := http.StatusInternalServerError
		message := "Internal Server Error"
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
			if msg, ok := he.Message.(string); ok {
				message = msg
			}
		} else {
			message = err.Error()
		}

		if c.Response().Committed {
			return
		}

		// Return JSON for API endpoints
		if strings.HasPrefix(c.Request().URL.Path, "/api/") {
			c.JSON(code, map[string]interface{}{
				"error":   http.StatusText(code),
				"message": message,
			})
			return
		}

		if code == http.StatusNotFound {
			if renderErr := tr.RenderWithStatus(c, "404.html", http.StatusNotFound, nil); renderErr != nil {
				c.HTML(http.StatusNotFound, "<h1>404 - Not Found</h1>")
			}
			return
		}

		c.HTML(code, fmt.Sprintf(
			"<h1>%d - %s</h1><p>%s</p><p><a href=\"/\">Back to home</a></p>",
			code, http.StatusText(code), message))
	}
}

// serveEmbedThumbnail returns a handler that serves a placeholder thumbnail
func serveEmbedThumbnail() echo.HandlerFunc {
	// 1x1 transparent PNG
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x62, 0x00, 0x00, 0x00, 0x02,
		0x00, 0x01, 0xe5, 0x27, 0xde, 0xfc, 0x00, 0x00,
		0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42,
		0x60, 0x82,
	}
	return func(c echo.Context) error {
		return c.Blob(http.StatusOK, "image/png", png)
	}
}

// serveEmbedDocument returns a handler that serves a placeholder for document viewing
func serveEmbedDocument() echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.HTML(http.StatusOK, "<html><body><p>Document viewing is not available in the demo.</p></body></html>")
	}
}
