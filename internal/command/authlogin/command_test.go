package authlogin

import (
	"context"
	"testing"
	"time"

	"github.com/aaronhurt/vagaro-sync/internal/browser"
	"github.com/aaronhurt/vagaro-sync/internal/storage"
	"github.com/aaronhurt/vagaro-sync/internal/testutil"
)

type fakeBackend struct {
	bundle storage.AuthBundle
}

func (f fakeBackend) Authenticate(context.Context) (storage.AuthBundle, error) {
	return f.bundle, nil
}

type fakeBrowserFactory struct {
	timeout time.Duration
	backend browserAuthenticator
}

func (f *fakeBrowserFactory) New(opts browser.ChromeOptions) (browserAuthenticator, error) {
	f.timeout = opts.Timeout
	return f.backend, nil
}

func TestCommandRunStoresAuthenticatedBundle(t *testing.T) {
	var (
		saved storage.AuthBundle
	)

	storeSave := func(_ context.Context, bundle storage.AuthBundle) error {
		saved = bundle
		return nil
	}

	backend := fakeBackend{
		bundle: storage.AuthBundle{SUToken: testutil.ValidJWT(t)},
	}
	factory := &fakeBrowserFactory{backend: backend}
	cmd := &Command{
		authStore:      authStoreStub{save: storeSave},
		browserFactory: factory,
	}

	if err := cmd.Run(context.Background(), []string{"-timeout=5m"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if factory.timeout != 5*time.Minute {
		t.Fatalf("timeout = %s, want %s", factory.timeout, 5*time.Minute)
	}
	if saved.SUToken != backend.bundle.SUToken {
		t.Fatalf("saved bundle = %+v", saved)
	}
}

func TestCommandRunRejectsInvalidCapturedToken(t *testing.T) {
	t.Parallel()

	saved := false
	cmd := &Command{
		authStore: authStoreStub{
			save: func(context.Context, storage.AuthBundle) error {
				saved = true
				return nil
			},
		},
		browserFactory: &fakeBrowserFactory{
			backend: fakeBackend{bundle: storage.AuthBundle{SUToken: "not-a-jwt"}},
		},
	}

	if err := cmd.Run(context.Background(), nil); err == nil {
		t.Fatal("expected invalid JWT error")
	}
	if saved {
		t.Fatal("expected invalid captured token not to be saved")
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
