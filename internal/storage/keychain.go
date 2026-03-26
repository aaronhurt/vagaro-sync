package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var errAuthBundleNotFound = errors.New("auth bundle not found")

type commandRunner func(context.Context, ...string) ([]byte, error)

type securityCommandError struct {
	err    error
	output string
}

func (e *securityCommandError) Error() string {
	output := strings.TrimSpace(e.output)
	if output == "" {
		return e.err.Error()
	}

	return fmt.Sprintf("%v: %s", e.err, output)
}

func (e *securityCommandError) Unwrap() error {
	return e.err
}

// KeychainStore persists authentication bundles in the macOS keychain.
type KeychainStore struct {
	service string
	account string
	run     commandRunner
}

// NewKeychainStore returns a macOS keychain-backed auth store.
func NewKeychainStore(service string, account string) *KeychainStore {
	return &KeychainStore{
		service: service,
		account: account,
		run:     runSecurityCommand,
	}
}

// Load retrieves the auth bundle from the macOS keychain.
func (s *KeychainStore) Load(ctx context.Context) (AuthBundle, error) {
	output, err := s.run(ctx, "find-generic-password", "-w", "-s", s.service, "-a", s.account)
	if err != nil {
		if isItemNotFound(err) {
			return AuthBundle{}, errAuthBundleNotFound
		}

		return AuthBundle{}, fmt.Errorf("load auth bundle from keychain: %w", err)
	}

	var bundle AuthBundle
	if err := json.Unmarshal(output, &bundle); err != nil {
		return AuthBundle{}, fmt.Errorf("decode auth bundle from keychain: %w", err)
	}

	return bundle, nil
}

// Save stores the auth bundle in the macOS keychain.
func (s *KeychainStore) Save(ctx context.Context, bundle AuthBundle) error {
	payload, err := json.Marshal(bundle)
	if err != nil {
		return fmt.Errorf("encode auth bundle for keychain: %w", err)
	}

	_, err = s.run(
		ctx,
		"add-generic-password",
		"-U",
		"-s",
		s.service,
		"-a",
		s.account,
		"-w",
		string(payload),
	)
	if err != nil {
		return fmt.Errorf("save auth bundle to keychain: %w", err)
	}

	return nil
}

// Delete removes the auth bundle from the macOS keychain.
func (s *KeychainStore) Delete(ctx context.Context) error {
	_, err := s.run(ctx, "delete-generic-password", "-s", s.service, "-a", s.account)
	if err != nil && !isItemNotFound(err) {
		return fmt.Errorf("delete auth bundle from keychain: %w", err)
	}

	return nil
}

func runSecurityCommand(ctx context.Context, args ...string) ([]byte, error) {
	// #nosec G204 -- the command name is fixed and args are assembled only from internal call sites.
	cmd := exec.CommandContext(ctx, "security", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, &securityCommandError{
			err:    err,
			output: string(output),
		}
	}

	return output, nil
}

func isItemNotFound(err error) bool {
	var commandErr *securityCommandError
	if !errors.As(err, &commandErr) {
		return false
	}

	return strings.Contains(commandErr.output, "could not be found in the keychain")
}
