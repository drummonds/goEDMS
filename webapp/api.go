package webapp

import (
	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

// GetAPIBaseURL returns the configured API base URL
// It reads from window.godocsConfig.apiURL if available,
// otherwise falls back to empty string (relative URLs)
func GetAPIBaseURL() string {
	// Check if config is available in browser
	if !app.IsClient {
		return "" // Server-side rendering - use relative URLs
	}

	// Try to get API URL from global config
	config := app.Window().Get("godocsConfig")
	if config.Truthy() {
		apiURL := config.Get("apiURL")
		if apiURL.Truthy() {
			url := apiURL.String()
			// Ensure no trailing slash
			if len(url) > 0 && url[len(url)-1] == '/' {
				return url[:len(url)-1]
			}
			return url
		}
	}

	// Fallback to relative URLs (same origin)
	return ""
}

// BuildAPIURL constructs a full API URL from a path
// Example: BuildAPIURL("/api/documents/latest") -> "http://backend:8000/api/documents/latest"
// or just "/api/documents/latest" if using relative URLs
func BuildAPIURL(path string) string {
	baseURL := GetAPIBaseURL()
	if baseURL == "" {
		return path // Relative URL
	}
	return baseURL + path
}

// Job represents a background job
type Job struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Progress    int    `json:"progress"`
	CurrentStep string `json:"currentStep"`
	TotalSteps  int    `json:"totalSteps"`
	Message     string `json:"message"`
	Error       string `json:"error,omitempty"`
	Result      string `json:"result,omitempty"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	StartedAt   string `json:"startedAt,omitempty"`
	CompletedAt string `json:"completedAt,omitempty"`
}

// LogLevel represents log severity levels matching slog
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// Log sends a structured log message to the backend using slog-compatible format
// This function encapsulates the slog interface for frontend logging
func Log(ctx app.Context, level LogLevel, message string, attrs map[string]interface{}) {
	// Always log to browser console for development
	if app.IsClient {
		consoleMethod := "log"
		switch level {
		case LogLevelError:
			consoleMethod = "error"
		case LogLevelWarn:
			consoleMethod = "warn"
		case LogLevelInfo:
			consoleMethod = "info"
		case LogLevelDebug:
			consoleMethod = "debug"
		}
		app.Window().Get("console").Call(consoleMethod, "["+string(level)+"]", message, attrs)
	}

	// Send to backend for persistent logging (async, non-blocking)
	ctx.Async(func() {
		payload := map[string]interface{}{
			"level":   string(level),
			"message": message,
			"attrs":   attrs,
		}

		// Convert to JSON string
		jsonStr := app.Window().Get("JSON").Call("stringify", payload).String()

		// Send to backend
		fetchOptions := map[string]interface{}{
			"method": "POST",
			"headers": map[string]interface{}{
				"Content-Type": "application/json",
			},
			"body": jsonStr,
		}

		apiURL := BuildAPIURL("/api/log")

		// Send with error handling for debugging
		promise := app.Window().Call("fetch", apiURL, fetchOptions)

		// Add error handler
		promise.Call("catch", app.FuncOf(func(this app.Value, args []app.Value) any {
			// Log fetch errors to console for debugging
			if len(args) > 0 {
				app.Window().Get("console").Call("error",
					"[Frontend Log] Failed to send log to backend:",
					args[0].String(),
					"URL:", apiURL)
			}
			return nil
		}))

		// Add success handler to verify delivery
		promise.Call("then", app.FuncOf(func(this app.Value, args []app.Value) any {
			if len(args) > 0 {
				response := args[0]
				status := response.Get("status").Int()
				if status != 200 {
					app.Window().Get("console").Call("warn",
						"[Frontend Log] Backend log endpoint returned non-200 status:",
						status, "URL:", apiURL)
				}
			}
			return nil
		}))
	})
}

// LogError logs an error message with structured attributes
func LogError(ctx app.Context, message string, attrs map[string]interface{}) {
	Log(ctx, LogLevelError, message, attrs)
}

// LogWarn logs a warning message with structured attributes
func LogWarn(ctx app.Context, message string, attrs map[string]interface{}) {
	Log(ctx, LogLevelWarn, message, attrs)
}

// LogInfo logs an info message with structured attributes
func LogInfo(ctx app.Context, message string, attrs map[string]interface{}) {
	Log(ctx, LogLevelInfo, message, attrs)
}

// LogDebug logs a debug message with structured attributes
func LogDebug(ctx app.Context, message string, attrs map[string]interface{}) {
	Log(ctx, LogLevelDebug, message, attrs)
}
