//go:build integration

package vagaro

import (
	"context"
	"testing"
	"time"

	"github.com/aaronhurt/vagaro-sync/internal/storage"
)

func TestLiveFetchUpcomingAppointments(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store := storage.NewKeychainStore(storage.DefaultKeychainService, storage.DefaultKeychainAccount)
	bundle, err := store.Load(ctx)
	if err != nil {
		t.Skipf("load auth bundle: %v", err)
	}
	if err := ValidateAuthBundle(bundle, time.Now().UTC()); err != nil {
		t.Skipf("validate auth bundle: %v", err)
	}

	client, err := NewClient(bundle)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	appointments, err := client.FetchUpcomingAppointments(ctx, 5)
	if err != nil {
		t.Fatalf("FetchUpcomingAppointments() error = %v", err)
	}
	if appointments == nil {
		t.Fatal("expected non-nil appointments slice")
	}
}
