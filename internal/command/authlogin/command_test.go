package authlogin

import (
	"context"
	"testing"
	"time"

	"github.com/aaronhurt/vagaro-sync/internal/browser"
	"github.com/aaronhurt/vagaro-sync/internal/storage"
)

type fakeBackend struct{}

func (f fakeBackend) Authenticate(context.Context, string) (storage.AuthBundle, error) {
	return storage.AuthBundle{}, nil
}

func TestCommandRunStoresAuthenticatedBundle(t *testing.T) {
	var (
		gotTimeout time.Duration
		saved      storage.AuthBundle
		probed     storage.AuthBundle
	)

	origNewChromeBackend := newChromeBackend
	origLoginWithBackend := loginWithBackend
	origProbeAppointments := probeAppointments
	t.Cleanup(func() {
		newChromeBackend = origNewChromeBackend
		loginWithBackend = origLoginWithBackend
		probeAppointments = origProbeAppointments
	})

	storeSave := func(_ context.Context, bundle storage.AuthBundle) error {
		saved = bundle
		return nil
	}

	cmd := &Command{
		AuthStore: authStoreStub{
			save: storeSave,
		},
	}

	newChromeBackend = func(opts browser.ChromeOptions) (browser.Backend, error) {
		gotTimeout = opts.Timeout
		return fakeBackend{}, nil
	}
	loginWithBackend = func(_ context.Context, backend browser.Backend) (storage.AuthBundle, error) {
		if _, ok := backend.(fakeBackend); !ok {
			t.Fatalf("backend = %#v, want fakeBackend", backend)
		}

		return storage.AuthBundle{SUToken: "token"}, nil
	}
	probeAppointments = func(_ context.Context, bundle storage.AuthBundle) error {
		probed = bundle
		return nil
	}

	if err := cmd.Run(context.Background(), []string{"-timeout=5m"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if gotTimeout != 5*time.Minute {
		t.Fatalf("timeout = %s, want %s", gotTimeout, 5*time.Minute)
	}
	if saved.SUToken != "token" {
		t.Fatalf("saved bundle = %+v", saved)
	}
	if probed.SUToken != "token" {
		t.Fatalf("probed bundle = %+v", probed)
	}
}

type authStoreStub struct {
	save func(context.Context, storage.AuthBundle) error
}

func (s authStoreStub) Load(context.Context) (storage.AuthBundle, error) {
	return storage.AuthBundle{}, nil
}

func (s authStoreStub) Save(ctx context.Context, bundle storage.AuthBundle) error {
	return s.save(ctx, bundle)
}

func (s authStoreStub) Delete(context.Context) error {
	return nil
}
