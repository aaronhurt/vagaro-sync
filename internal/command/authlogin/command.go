// Package authlogin implements the auth-login CLI command.
package authlogin

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aaronhurt/vagaro-sync/internal/auth"
	"github.com/aaronhurt/vagaro-sync/internal/browser"
	"github.com/aaronhurt/vagaro-sync/internal/storage"
	"github.com/aaronhurt/vagaro-sync/internal/vagaro"
)

type authStore interface {
	Save(context.Context, storage.AuthBundle) error
}

type browserAuthenticator interface {
	Authenticate(context.Context, string) (storage.AuthBundle, error)
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
	now            func() time.Time
}

// NewCommand constructs the auth-login command.
func NewCommand(store *storage.KeychainStore) *Command {
	return &Command{
		authStore:      store,
		browserFactory: chromeBackendFactory{},
		now:            time.Now,
	}
}

// Run executes the auth-login command.
func (c *Command) Run(ctx context.Context, args []string) error {
	if c.browserFactory == nil {
		c.browserFactory = chromeBackendFactory{}
	}
	if c.now == nil {
		c.now = time.Now
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

	bundle, err := auth.Login(ctx, backend)
	if err != nil {
		return err
	}
	if err := vagaro.ValidateAuthBundle(bundle, c.now().UTC()); err != nil {
		return err
	}

	if err := c.authStore.Save(ctx, bundle); err != nil {
		return err
	}

	_, err = fmt.Fprintln(os.Stdout, "authentication stored")
	return err
}
