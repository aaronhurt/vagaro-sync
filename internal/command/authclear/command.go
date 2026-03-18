// Package authclear implements the auth-clear CLI command.
package authclear

import (
	"context"
	"fmt"
	"os"

	"github.com/aaronhurt/vagaro-sync/internal/storage"
)

// Command runs the auth-clear flow.
type Command struct {
	AuthStore storage.AuthStore
}

// Run executes the auth-clear command.
func (c *Command) Run(ctx context.Context, _ []string) error {
	if err := c.AuthStore.Delete(ctx); err != nil {
		return err
	}

	_, err := fmt.Fprintln(os.Stdout, "authentication cleared")
	return err
}
