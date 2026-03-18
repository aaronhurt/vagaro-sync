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
)

var newChromeBackend = func(opts browser.ChromeOptions) (browser.Backend, error) {
	return browser.NewChromeBackend(opts)
}

var loginWithBackend = auth.Login

var probeAppointments = auth.ProbeAppointments

// Command runs the auth-login flow.
type Command struct {
	AuthStore storage.AuthStore
}

// Run executes the auth-login command.
func (c *Command) Run(ctx context.Context, args []string) error {
	cmd := flag.NewFlagSet("auth-login", flag.ContinueOnError)
	cmd.SetOutput(os.Stderr)

	timeout := cmd.Duration("timeout", 10*time.Minute, "maximum time to wait for browser-assisted login")
	if err := cmd.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}

		return err
	}

	backend, err := newChromeBackend(browser.ChromeOptions{
		Timeout: *timeout,
	})
	if err != nil {
		return err
	}

	bundle, err := loginWithBackend(ctx, backend)
	if err != nil {
		return err
	}

	if err := probeAppointments(ctx, bundle); err != nil {
		return err
	}

	if err := c.AuthStore.Save(ctx, bundle); err != nil {
		return err
	}

	_, err = fmt.Fprintln(os.Stdout, "authentication stored")
	return err
}
