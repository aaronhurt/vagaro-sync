package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadReturnsEmptyStateForMissingFile(t *testing.T) {
	t.Parallel()

	store := NewFileStore(filepath.Join(t.TempDir(), "state.json"))
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(got.Appointments) != 0 {
		t.Fatalf("Appointments length = %d, want 0", len(got.Appointments))
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewFileStore(filepath.Join(t.TempDir(), "state.json"))
	now := time.Date(2026, time.March, 17, 12, 0, 0, 0, time.UTC)
	want := SyncState{
		Appointments: map[string]AppointmentState{
			"apt-1": {
				EventID:    "calendar-event-1",
				SourceHash: "hash-1",
				UpdatedAt:  now,
			},
		},
	}

	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.Appointments["apt-1"] != want.Appointments["apt-1"] {
		t.Fatalf("Appointments[apt-1] = %+v, want %+v", got.Appointments["apt-1"], want.Appointments["apt-1"])
	}
}
