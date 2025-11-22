package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestJobsAPI tests job tracking endpoints
func TestJobsAPI(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("GetJob_NotFound", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/jobs/01NONEXISTENT00000000000000", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should return 400 Bad Request or 404 Not Found for non-existent job
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("Expected status 400 or 404 for non-existent job, got %d", rec.Code)
		}
	})

	t.Run("GetJob_InvalidID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/jobs/invalid-id", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should return 400 Bad Request or 404 Not Found for invalid job ID format
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("Expected status 400 or 404 for invalid job ID, got %d", rec.Code)
		}
	})

	t.Run("GetJob_Success", func(t *testing.T) {
		// First, trigger a job by running ingest or clean
		ingestReq := httptest.NewRequest(http.MethodPost, "/api/ingest", nil)
		ingestRec := httptest.NewRecorder()
		e.ServeHTTP(ingestRec, ingestReq)

		if ingestRec.Code != http.StatusOK {
			t.Skipf("Could not create job via ingest: %d", ingestRec.Code)
		}

		// Parse response to get job ID
		var ingestResponse map[string]interface{}
		err := json.Unmarshal(ingestRec.Body.Bytes(), &ingestResponse)
		assert.NoError(t, err, "Should parse ingest response")

		jobID, ok := ingestResponse["jobId"].(string)
		if !ok || jobID == "" {
			t.Skip("Ingest did not return a job ID")
		}

		// Now fetch the job
		req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+jobID, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "Should return 200 OK for valid job ID")

		var job map[string]interface{}
		err = json.Unmarshal(rec.Body.Bytes(), &job)
		assert.NoError(t, err, "Response should be valid JSON")

		// Verify job structure
		assert.NotEmpty(t, job["id"], "Job should have id field")
		assert.NotEmpty(t, job["type"], "Job should have type field")
		assert.NotEmpty(t, job["status"], "Job should have status field")
		assert.Contains(t, job, "progress", "Job should have progress field")
		assert.Contains(t, job, "createdAt", "Job should have createdAt field")
		assert.Contains(t, job, "updatedAt", "Job should have updatedAt field")

		t.Logf("Retrieved job: type=%v, status=%v, progress=%v",
			job["type"], job["status"], job["progress"])
	})

	t.Run("GetRecentJobs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "Should return 200 OK")

		var jobs []interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &jobs)
		assert.NoError(t, err, "Response should be valid JSON array")

		t.Logf("Retrieved %d recent jobs", len(jobs))
	})

	t.Run("GetActiveJobs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/jobs/active", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "Should return 200 OK")

		var jobs []interface{}
		err := json.Unmarshal(rec.Body.Bytes(), &jobs)
		assert.NoError(t, err, "Response should be valid JSON array")

		t.Logf("Retrieved %d active jobs", len(jobs))
	})
}

// TestFrontendLoggingAPI tests the frontend logging endpoint
func TestFrontendLoggingAPI(t *testing.T) {
	e, _, cleanup := setupTestServer(t)
	defer cleanup()

	t.Run("LogFromFrontend_ErrorLevel", func(t *testing.T) {
		payload := map[string]interface{}{
			"level":   "error",
			"message": "Test error from frontend",
			"attrs": map[string]interface{}{
				"component": "TestComponent",
				"action":    "testAction",
			},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "Should return 200 OK")

		var response map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err, "Response should be valid JSON")
		assert.Equal(t, "logged", response["status"], "Should return logged status")
	})

	t.Run("LogFromFrontend_WarnLevel", func(t *testing.T) {
		payload := map[string]interface{}{
			"level":   "warn",
			"message": "Test warning from frontend",
			"attrs": map[string]interface{}{
				"component": "TestComponent",
			},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "Should return 200 OK")
	})

	t.Run("LogFromFrontend_InfoLevel", func(t *testing.T) {
		payload := map[string]interface{}{
			"level":   "info",
			"message": "Test info from frontend",
			"attrs":   map[string]interface{}{},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "Should return 200 OK")
	})

	t.Run("LogFromFrontend_DebugLevel", func(t *testing.T) {
		payload := map[string]interface{}{
			"level":   "debug",
			"message": "Test debug from frontend",
			"attrs": map[string]interface{}{
				"detail": "verbose details",
			},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "Should return 200 OK")
	})

	t.Run("LogFromFrontend_InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code, "Should return 400 Bad Request for invalid JSON")
	})

	t.Run("LogFromFrontend_MissingMessage", func(t *testing.T) {
		payload := map[string]interface{}{
			"level": "error",
			"attrs": map[string]interface{}{},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should accept empty message (currently implementation allows it)
		if rec.Code != http.StatusOK && rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 200 or 400 for missing message, got %d", rec.Code)
		}
	})

	t.Run("LogFromFrontend_InvalidLevel", func(t *testing.T) {
		payload := map[string]interface{}{
			"level":   "invalid_level",
			"message": "Test with invalid level",
			"attrs":   map[string]interface{}{},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should handle gracefully - current implementation defaults to info level
		assert.Equal(t, http.StatusOK, rec.Code, "Should return 200 OK even with invalid level")
	})

	t.Run("LogFromFrontend_ComplexAttributes", func(t *testing.T) {
		payload := map[string]interface{}{
			"level":   "error",
			"message": "Complex attributes test",
			"attrs": map[string]interface{}{
				"component":  "EditPage",
				"action":     "save",
				"documentId": "01ABC123DEF456GHI789JKL000",
				"errorCode":  500,
				"nested": map[string]interface{}{
					"key": "value",
				},
			},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code, "Should handle complex attributes")
	})

	t.Run("LogFromFrontend_MissingLevel", func(t *testing.T) {
		payload := map[string]interface{}{
			"message": "Test without level",
			"attrs":   map[string]interface{}{},
		}
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// Should default to info level or return OK
		assert.Equal(t, http.StatusOK, rec.Code, "Should handle missing level gracefully")
	})
}
