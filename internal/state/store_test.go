package state

import (
	"os"
	"path/filepath"
	"strings"
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

func TestLoadSelfHealsMalformedState(t *testing.T) {
	t.Parallel()

	statePath := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(statePath, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	store := NewFileStore(statePath)
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(got.Appointments) != 0 {
		t.Fatalf("Appointments length = %d, want 0", len(got.Appointments))
	}
}

func TestSaveDoesNotLeaveTemporaryFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := NewFileStore(filepath.Join(dir, "state.json"))
	if err := store.Save(SyncState{Appointments: map[string]AppointmentState{}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("os.ReadDir() error = %v", err)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".state-") {
			t.Fatalf("unexpected temporary file %q left behind", entry.Name())
		}
	}
}
