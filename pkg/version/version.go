package version

import (
	"sync"
)

var (
	// Version is set at build time via -ldflags
	Version = "dev"

	// cmdVersion stores the version from cmd/troupe if set via ldflags
	cmdVersion string
	once       sync.Once
)

// SetCmdVersion is called by cmd/troupe to register its version
// This allows pkg/version to know if cmd/troupe.Version was set via ldflags
func SetCmdVersion(v string) {
	once.Do(func() {
		cmdVersion = v
	})
}

// GetVersion returns the app version
// It prefers cmdVersion (from cmd/troupe.Version set via old CI ldflags)
// Otherwise returns pkg/version.Version
func GetVersion() string {
	if cmdVersion != "" {
		return cmdVersion
	}
	return Version
}
