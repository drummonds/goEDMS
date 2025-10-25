package webapp

import (
	"testing"
)

// TestNotFoundPageStructure tests the NotFoundPage component structure
func TestNotFoundPageStructure(t *testing.T) {
	page := &NotFoundPage{}

	// Verify component exists
	if page == nil {
		t.Error("NotFoundPage component should not be nil")
	}

	// Test that Render returns a valid UI
	ui := page.Render()
	if ui == nil {
		t.Error("Render should return a valid UI component")
	}

	t.Log("NotFoundPage component structure verified")
}

// TestNotFoundPageRender tests that the component can be rendered
func TestNotFoundPageRender(t *testing.T) {
	page := &NotFoundPage{}

	ui := page.Render()
	if ui == nil {
		t.Error("NotFoundPage Render should not return nil")
	}

	t.Log("NotFoundPage renders successfully")
}

// TestAppRendersNotFoundPage tests that the App component uses NotFoundPage for unknown routes
func TestAppRendersNotFoundPage(t *testing.T) {
	// This test documents the expected behavior:
	// Unknown routes should display NotFoundPage instead of HomePage

	// Note: We can't easily test the actual routing without a full WASM environment,
	// but we can document the expected behavior

	t.Log("Expected behavior: Unknown routes should render NotFoundPage")
	t.Log("This is configured in app.go default case: return &NotFoundPage{}")
}
