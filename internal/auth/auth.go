// Package auth handles browser-assisted authentication.
package auth

import (
	"context"

	"github.com/aaronhurt/vagaro-sync/internal/storage"
)

const loginURL = "https://www.vagaro.com/login"

type browserAuthenticator interface {
	Authenticate(context.Context, string) (storage.AuthBundle, error)
}

// Login launches provider-agnostic browser authentication and returns the captured session bundle.
func Login(ctx context.Context, backend browserAuthenticator) (storage.AuthBundle, error) {
	return backend.Authenticate(ctx, loginURL)
}
