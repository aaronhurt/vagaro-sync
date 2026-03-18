// Package storage persists authentication state for later sync runs.
package storage

import (
	"context"
	"time"
)

// Default keychain identifiers are used for storing the Vagaro auth bundle.
const (
	DefaultKeychainService = "github.com/aaronhurt/vagaro-sync"
	DefaultKeychainAccount = "default"
)

// AuthBundle stores the captured browser session needed for Vagaro API access.
type AuthBundle struct {
	CapturedAt time.Time `json:"captured_at"`
	SUToken    string    `json:"s_utkn"`
	UserAgent  string    `json:"user_agent"`
}

// AuthStore persists and retrieves authentication bundles.
type AuthStore interface {
	Load(context.Context) (AuthBundle, error)
	Save(context.Context, AuthBundle) error
	Delete(context.Context) error
}
