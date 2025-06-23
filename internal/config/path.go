// Package config provides configuration utilities for the application.
package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ExpandPath expands ~ and environment variables in a file path.
// It handles both ~ for home directory and $VAR style environment variables.
func ExpandPath(path string) string {
	if path == "" {
		return path
	}

	// First expand tilde if present
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	} else if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			path = home
		}
	}

	// Then expand environment variables
	return os.ExpandEnv(path)
}
