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
	if err := client.ProbeSession(ctx); err != nil {
		t.Fatalf("ProbeSession() error = %v", err)
	}

	appointments, err := client.FetchUpcomingAppointments(ctx, 5)
	if err != nil {
		t.Fatalf("FetchUpcomingAppointments() error = %v", err)
	}
	if appointments == nil {
		t.Fatal("expected non-nil appointments slice")
	}

	for idx, appointment := range appointments {
		if appointment.AppointmentID == "" {
			t.Fatalf("appointments[%d] missing AppointmentID: %+v", idx, appointment)
		}
		if appointment.SourceHash == "" {
			t.Fatalf("appointments[%d] missing SourceHash: %+v", idx, appointment)
		}
		if appointment.StartTimeUTC.IsZero() {
			t.Fatalf("appointments[%d] missing StartTimeUTC: %+v", idx, appointment)
		}
		if appointment.EndTimeUTC.Before(appointment.StartTimeUTC) {
			t.Fatalf("appointments[%d] has EndTimeUTC before StartTimeUTC: %+v", idx, appointment)
		}
	}
}
