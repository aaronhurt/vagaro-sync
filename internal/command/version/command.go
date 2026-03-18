// Package version implements the version CLI command.
package version

import (
	"context"
	"io"
	"os"
)

// Command prints the configured version string.
type Command struct {
	Version string
}

// Run executes the version command.
func (c *Command) Run(_ context.Context, _ []string) error {
	version := c.Version
	if version == "" {
		version = "dev"
	}

	_, err := io.WriteString(os.Stdout, version+"\n")
	return err
}
