// Package authclear implements the auth-clear CLI command.
package authclear

import (
	"context"
	"fmt"
	"os"

	"github.com/aaronhurt/vagaro-sync/internal/storage"
)

type authStore interface {
	Delete(context.Context) error
}

// Command runs the auth-clear flow.
type Command struct {
	authStore authStore
}

// NewCommand constructs the auth-clear command.
func NewCommand(store *storage.KeychainStore) *Command {
	return &Command{authStore: store}
}

// Run executes the auth-clear command.
func (c *Command) Run(ctx context.Context, _ []string) error {
	if err := c.authStore.Delete(ctx); err != nil {
		return err
	}

	_, err := fmt.Fprintln(os.Stdout, "authentication cleared")
	return err
}
