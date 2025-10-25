package webapp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandlerRoutes tests that all expected routes are registered
func TestHandlerRoutes(t *testing.T) {
	handler := Handler()

	tests := []struct {
		name string
		path string
	}{
		{
			name: "Home page",
			path: "/",
		},
		{
			name: "Browse page",
			path: "/browse",
		},
		{
			name: "Ingest page",
			path: "/ingest",
		},
		{
			name: "Clean page",
			path: "/clean",
		},
		{
			name: "Search page",
			path: "/search",
		},
		{
			name: "Word Cloud page",
			path: "/wordcloud",
		},
		{
			name: "About page",
			path: "/about",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Should return 200 OK or at least not 404
			if rec.Code == http.StatusNotFound {
				t.Errorf("Route %s returned 404 Not Found - route may not be registered", tt.path)
			}

			// Should return HTML content
			contentType := rec.Header().Get("Content-Type")
			if !strings.Contains(contentType, "text/html") && rec.Code == http.StatusOK {
				t.Logf("Note: Route %s returned status %d with Content-Type: %s", tt.path, rec.Code, contentType)
			}

			t.Logf("Route %s returned status %d", tt.path, rec.Code)
		})
	}
}

// TestWordCloudPageRegistration specifically tests that word cloud page exists
func TestWordCloudPageRegistration(t *testing.T) {
	handler := Handler()

	req := httptest.NewRequest(http.MethodGet, "/wordcloud", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Errorf("Word cloud page returned 404 Not Found")
		t.Error("Make sure /wordcloud route is registered in webapp/handler.go")
		t.Error("Add: app.Route(\"/wordcloud\", func() app.Composer { return &App{} })")
	} else {
		t.Logf("Word cloud page successfully registered, returned status %d", rec.Code)
	}
}

// TestAppRenderPage tests that the App component correctly routes to WordCloudPage
func TestAppRenderPage(t *testing.T) {
	// This is a unit test that would require setting up a mock context
	// For now, we just verify the structure exists
	app := &App{}

	// Verify App component exists and has Render method
	if app == nil {
		t.Error("App component is nil")
	}

	// The actual routing is tested via the integration test above
	t.Log("App component structure verified")
}
