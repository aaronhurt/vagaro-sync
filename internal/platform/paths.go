// Package platform resolves application-specific filesystem paths.
package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

const appName = "vagaro-sync"

// DefaultStatePath returns the default sync state file location for the current user.
func DefaultStatePath() (string, error) {
	rootDir, err := appRootDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(rootDir, "state.json"), nil
}

// EnsureParentDir creates the parent directory for path when needed.
func EnsureParentDir(path string) error {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return fmt.Errorf("create parent directory for %q: %w", path, err)
	}

	return nil
}

func appRootDir() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}

	return filepath.Join(baseDir, appName), nil
}
