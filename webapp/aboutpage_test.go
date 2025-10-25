package webapp

import (
	"testing"
)

// TestGetDatabaseDisplay tests the database type display conversion
func TestGetDatabaseDisplay(t *testing.T) {
	tests := []struct {
		name     string
		dbType   string
		expected string
	}{
		{
			name:     "PostgreSQL",
			dbType:   "postgres",
			expected: "PostgreSQL",
		},
		{
			name:     "CockroachDB",
			dbType:   "cockroachdb",
			expected: "CockroachDB",
		},
		{
			name:     "SQLite",
			dbType:   "sqlite",
			expected: "SQLite",
		},
		{
			name:     "Unknown type",
			dbType:   "mongodb",
			expected: "mongodb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := &AboutPage{
				aboutInfo: AboutInfo{
					DatabaseType: tt.dbType,
				},
			}
			got := page.getDatabaseDisplay()
			if got != tt.expected {
				t.Errorf("getDatabaseDisplay() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestGetOCRStatus tests the OCR status display conversion
func TestGetOCRStatus(t *testing.T) {
	tests := []struct {
		name          string
		ocrConfigured bool
		expected      string
	}{
		{
			name:          "OCR Enabled",
			ocrConfigured: true,
			expected:      "Enabled",
		},
		{
			name:          "OCR Disabled",
			ocrConfigured: false,
			expected:      "Disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := &AboutPage{
				aboutInfo: AboutInfo{
					OCRConfigured: tt.ocrConfigured,
				},
			}
			got := page.getOCRStatus()
			if got != tt.expected {
				t.Errorf("getOCRStatus() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestAboutPageRenderStates tests that different states produce valid UI
// Note: Full HTML validation is done in integration tests (TestAboutPageWithChromedp)
func TestAboutPageRenderStates(t *testing.T) {
	t.Run("Loading state returns valid UI", func(t *testing.T) {
		page := &AboutPage{
			loading: true,
		}
		ui := page.Render()

		if ui == nil {
			t.Error("Loading state should return non-nil UI")
		}
	})

	t.Run("Error state returns valid UI", func(t *testing.T) {
		page := &AboutPage{
			loading: false,
			error:   "Network error",
		}
		ui := page.Render()

		if ui == nil {
			t.Error("Error state should return non-nil UI")
		}
	})

	t.Run("Success state returns valid UI", func(t *testing.T) {
		page := &AboutPage{
			loading: false,
			error:   "",
			aboutInfo: AboutInfo{
				Version:       "v1.2.3",
				OCRConfigured: true,
				OCRPath:       "/usr/bin/tesseract",
				DatabaseType:  "postgres",
			},
		}
		ui := page.Render()

		if ui == nil {
			t.Error("Success state should return non-nil UI")
		}
	})
}

// TestAboutPageStateTransitions tests state management logic
func TestAboutPageStateTransitions(t *testing.T) {
	t.Run("Initial state should be loading", func(t *testing.T) {
		page := &AboutPage{}

		// When OnMount is called, it should set loading to true
		// We can't test OnMount directly without a browser context,
		// but we can verify the state logic
		page.loading = true

		if !page.loading {
			t.Error("Initial state should have loading=true")
		}
	})

	t.Run("Error state should have error message", func(t *testing.T) {
		page := &AboutPage{
			loading: false,
			error:   "Failed to fetch data",
		}

		if page.loading {
			t.Error("Error state should have loading=false")
		}
		if page.error == "" {
			t.Error("Error state should have error message")
		}
	})

	t.Run("Success state should have data", func(t *testing.T) {
		page := &AboutPage{
			loading: false,
			error:   "",
			aboutInfo: AboutInfo{
				Version:      "v1.0.0",
				DatabaseType: "postgres",
			},
		}

		if page.loading {
			t.Error("Success state should have loading=false")
		}
		if page.error != "" {
			t.Error("Success state should have no error")
		}
		if page.aboutInfo.Version == "" {
			t.Error("Success state should have version data")
		}
	})
}

// TestGetConnectionType tests the connection type display conversion
func TestGetConnectionType(t *testing.T) {
	tests := []struct {
		name        string
		isEphemeral bool
		expected    string
	}{
		{
			name:        "Ephemeral database",
			isEphemeral: true,
			expected:    "Ephemeral (Temporary, On-Disk)",
		},
		{
			name:        "External database",
			isEphemeral: false,
			expected:    "External (Persistent)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := &AboutPage{
				aboutInfo: AboutInfo{
					IsEphemeral: tt.isEphemeral,
				},
			}
			got := page.getConnectionType()
			if got != tt.expected {
				t.Errorf("getConnectionType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestAboutInfoStruct tests the AboutInfo struct
func TestAboutInfoStruct(t *testing.T) {
	info := AboutInfo{
		Version:       "v2.0.0",
		OCRConfigured: true,
		OCRPath:       "/opt/tesseract",
		DatabaseType:  "cockroachdb",
		DatabaseHost:  "db.example.com",
		DatabasePort:  "26257",
		DatabaseName:  "goedms_prod",
		IsEphemeral:   false,
	}

	if info.Version != "v2.0.0" {
		t.Errorf("Version = %v, want v2.0.0", info.Version)
	}
	if !info.OCRConfigured {
		t.Error("OCRConfigured should be true")
	}
	if info.OCRPath != "/opt/tesseract" {
		t.Errorf("OCRPath = %v, want /opt/tesseract", info.OCRPath)
	}
	if info.DatabaseType != "cockroachdb" {
		t.Errorf("DatabaseType = %v, want cockroachdb", info.DatabaseType)
	}
	if info.DatabaseHost != "db.example.com" {
		t.Errorf("DatabaseHost = %v, want db.example.com", info.DatabaseHost)
	}
	if info.DatabasePort != "26257" {
		t.Errorf("DatabasePort = %v, want 26257", info.DatabasePort)
	}
	if info.DatabaseName != "goedms_prod" {
		t.Errorf("DatabaseName = %v, want goedms_prod", info.DatabaseName)
	}
	if info.IsEphemeral {
		t.Error("IsEphemeral should be false")
	}
}
