package auth

import (
	"context"
	"testing"

	"github.com/aaronhurt/vagaro-sync/internal/storage"
)

type fakeAuthenticator struct {
	loginURL string
	bundle   storage.AuthBundle
}

func (f *fakeAuthenticator) Authenticate(_ context.Context, loginURL string) (storage.AuthBundle, error) {
	f.loginURL = loginURL
	return f.bundle, nil
}

func TestLoginUsesVagaroLoginURL(t *testing.T) {
	t.Parallel()

	backend := &fakeAuthenticator{bundle: storage.AuthBundle{
		SUToken:   "token",
		UserAgent: "test-agent",
	}}

	got, err := Login(context.Background(), backend)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if got.SUToken != "token" {
		t.Fatalf("bundle = %+v", got)
	}
	if backend.loginURL != loginURL {
		t.Fatalf("loginURL = %q, want %q", backend.loginURL, loginURL)
	}
}
