// Package browser provides browser-backed authentication helpers.
package browser

import (
	"context"

	"github.com/aaronhurt/vagaro-sync/internal/storage"
)

// Backend captures authenticated browser state for later API use.
type Backend interface {
	Authenticate(context.Context, string) (storage.AuthBundle, error)
}
