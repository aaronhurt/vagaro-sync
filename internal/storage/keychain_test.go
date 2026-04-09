package storage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"testing"
	"time"
)

func TestKeychainStoreSaveUsesExpectedArguments(t *testing.T) {
	t.Parallel()

	var gotArgs []string
	var gotStdin []byte
	store := &KeychainStore{
		service: "service",
		account: "account",
		run: func(_ context.Context, stdin io.Reader, args ...string) ([]byte, error) {
			gotArgs = append([]string(nil), args...)
			if stdin != nil {
				var err error
				gotStdin, err = io.ReadAll(stdin)
				if err != nil {
					t.Fatalf("read stdin: %v", err)
				}
			}
			return nil, nil
		},
	}

	bundle := AuthBundle{
		CapturedAt: time.Date(2026, time.March, 17, 13, 0, 0, 0, time.UTC),
		SUToken:    "token",
	}
	if err := store.Save(context.Background(), bundle); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Payload must be passed via stdin, not as a -w argument.
	wantArgs := []string{
		"add-generic-password",
		"-U",
		"-s",
		"service",
		"-a",
		"account",
	}
	if !slices.Equal(gotArgs, wantArgs) {
		t.Fatalf("args = %q, want %q", gotArgs, wantArgs)
	}

	var saved AuthBundle
	if err := json.Unmarshal(gotStdin, &saved); err != nil {
		t.Fatalf("Unmarshal(stdin payload) error = %v", err)
	}
	if saved.SUToken != bundle.SUToken {
		t.Fatalf("saved SUToken = %q, want %q", saved.SUToken, bundle.SUToken)
	}
	if saved.UserAgent != "" {
		t.Fatalf("saved UserAgent = %q, want empty", saved.UserAgent)
	}
}

func TestKeychainStoreLoadDecodesBundle(t *testing.T) {
	t.Parallel()

	store := &KeychainStore{
		service: "service",
		account: "account",
		run: func(_ context.Context, _ io.Reader, _ ...string) ([]byte, error) {
			return []byte(`{"captured_at":"2026-03-17T13:00:00Z","s_utkn":"token","user_agent":"agent","cookies":[{"name":"session","value":"cookie","domain":"example.com","path":"/","expires_at":"2026-03-18T00:00:00Z"}]}`), nil
		},
	}

	got, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.SUToken != "token" {
		t.Fatalf("SUToken = %q, want token", got.SUToken)
	}
	if got.UserAgent != "agent" {
		t.Fatalf("UserAgent = %q, want agent", got.UserAgent)
	}
}

func TestKeychainStoreLoadReturnsNotFound(t *testing.T) {
	t.Parallel()

	store := &KeychainStore{
		service: "service",
		account: "account",
		run: func(_ context.Context, _ io.Reader, _ ...string) ([]byte, error) {
			return nil, &securityCommandError{
				err:    errors.New("security failed"),
				output: "security: secKeychainSearchCopyNext: the specified item could not be found in the keychain",
			}
		},
	}

	_, err := store.Load(context.Background())
	if !errors.Is(err, errAuthBundleNotFound) {
		t.Fatalf("Load() error = %v, want %v", err, errAuthBundleNotFound)
	}
}

func TestKeychainStoreDeleteIgnoresMissingItem(t *testing.T) {
	t.Parallel()

	store := &KeychainStore{
		service: "service",
		account: "account",
		run: func(_ context.Context, _ io.Reader, _ ...string) ([]byte, error) {
			return nil, &securityCommandError{
				err:    errors.New("security failed"),
				output: "security: secKeychainSearchCopyNext: the specified item could not be found in the keychain",
			}
		},
	}

	if err := store.Delete(context.Background()); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
}

func TestKeychainStoreDeleteReturnsNonNotFoundError(t *testing.T) {
	t.Parallel()

	store := &KeychainStore{
		service: "service",
		account: "account",
		run: func(_ context.Context, _ io.Reader, _ ...string) ([]byte, error) {
			return nil, &securityCommandError{
				err:    errors.New("security failed"),
				output: "security: access denied",
			}
		},
	}

	if err := store.Delete(context.Background()); err == nil {
		t.Fatal("expected delete error")
	}
}
