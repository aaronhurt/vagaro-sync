// Package state persists the local sync reconciliation state.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aaronhurt/vagaro-sync/internal/platform"
)

// AppointmentState stores the last synced calendar metadata for an appointment.
type AppointmentState struct {
	EventID    string    `json:"event_id"`
	SourceHash string    `json:"source_hash"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// SyncState stores the full local reconciliation state.
type SyncState struct {
	Appointments map[string]AppointmentState `json:"appointments"`
}

// LoadStatus reports non-fatal conditions encountered while reading state.
type LoadStatus struct {
	Corrupted bool
}

// FileStore reads and writes sync state as JSON on disk.
type FileStore struct {
	path string
}

// NewFileStore returns a file-backed sync state store rooted at path.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// Load returns the current sync state, treating a missing file as empty state.
func (s *FileStore) Load() (SyncState, LoadStatus, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return SyncState{Appointments: map[string]AppointmentState{}}, LoadStatus{}, nil
		}

		return SyncState{}, LoadStatus{}, fmt.Errorf("read sync state %q: %w", s.path, err)
	}

	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return SyncState{Appointments: map[string]AppointmentState{}}, LoadStatus{Corrupted: true}, nil
	}

	if state.Appointments == nil {
		state.Appointments = map[string]AppointmentState{}
	}

	return state, LoadStatus{}, nil
}

// Save writes the provided sync state to disk.
func (s *FileStore) Save(state SyncState) error {
	if state.Appointments == nil {
		state.Appointments = map[string]AppointmentState{}
	}

	if err := platform.EnsureParentDir(s.path); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode sync state %q: %w", s.path, err)
	}

	data = append(data, '\n')
	tempFile, err := os.CreateTemp(filepath.Dir(s.path), ".state-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary sync state for %q: %w", s.path, err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("write temporary sync state for %q: %w", s.path, err)
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("sync temporary state file for %q: %w", s.path, err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temporary state file for %q: %w", s.path, err)
	}

	if err := os.Rename(tempPath, s.path); err != nil {
		return fmt.Errorf("replace sync state %q: %w", s.path, err)
	}

	return nil
}
