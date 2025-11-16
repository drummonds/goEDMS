package build

import (
	"runtime/debug"
)

// Version is set via ldflags during build, or auto-detected from git
var Version = "development"

// GetVersion returns the version, attempting to auto-detect from build info if not set via ldflags
func GetVersion() string {
	// If version was set via ldflags (e.g., using Taskfile), return it
	if Version != "" && Version != "development" {
		return Version
	}

	// Try to auto-detect from build info (works with go install)
	if info, ok := debug.ReadBuildInfo(); ok {
		// Check for version from go.mod or VCS info
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}

		// Try to get version from VCS (Git) build settings
		var revision, modified string
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
			case "vcs.modified":
				modified = setting.Value
			}
		}

		if revision != "" {
			// Shorten git hash to 7 characters
			shortRev := revision
			if len(shortRev) > 7 {
				shortRev = shortRev[:7]
			}

			version := "v0.0.0-" + shortRev
			if modified == "true" {
				version += "-dirty"
			}
			return version
		}
	}

	return Version
}
