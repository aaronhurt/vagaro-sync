// Package authlogin implements the auth-login CLI command.
package authlogin

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aaronhurt/vagaro-sync/internal/browser"
	"github.com/aaronhurt/vagaro-sync/internal/storage"
	"github.com/aaronhurt/vagaro-sync/internal/vagaro"
)

type authStore interface {
	Save(context.Context, storage.AuthBundle) error
}

type browserAuthenticator interface {
	Authenticate(context.Context) (storage.AuthBundle, error)
}

type browserFactory interface {
	New(browser.ChromeOptions) (browserAuthenticator, error)
}

type chromeBackendFactory struct{}

func (chromeBackendFactory) New(opts browser.ChromeOptions) (browserAuthenticator, error) {
	return browser.NewChromeBackend(opts)
}

// Command runs the auth-login flow.
type Command struct {
	authStore      authStore
	browserFactory browserFactory
}

// NewCommand constructs the auth-login command.
func NewCommand(store *storage.KeychainStore) *Command {
	return &Command{
		authStore:      store,
		browserFactory: chromeBackendFactory{},
	}
}

// Run executes the auth-login command.
func (c *Command) Run(ctx context.Context, args []string) error {
	if c.browserFactory == nil {
		c.browserFactory = chromeBackendFactory{}
	}

	cmd := flag.NewFlagSet("auth-login", flag.ContinueOnError)
	cmd.SetOutput(os.Stderr)

	timeout := cmd.Duration("timeout", 10*time.Minute, "maximum time to wait for browser-assisted login")
	if err := cmd.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}

		return err
	}

	backend, err := c.browserFactory.New(browser.ChromeOptions{
		Timeout: *timeout,
	})
	if err != nil {
		return err
	}

	bundle, err := backend.Authenticate(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := vagaro.ValidateAuthBundle(bundle, now); err != nil {
		return err
	}
	remaining, err := vagaro.RemainingTokenLifetime(bundle, now)
	if err != nil {
		return err
	}

	if err := c.authStore.Save(ctx, bundle); err != nil {
		return err
	}

	_, err = fmt.Fprintf(
		os.Stdout,
		"authentication stored\nauth expires in: %s\n",
		vagaro.FormatTokenLifetime(remaining),
	)
	return err
}
