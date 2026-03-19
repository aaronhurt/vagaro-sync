package synccommand

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aaronhurt/vagaro-sync/internal/calendar"
	"github.com/aaronhurt/vagaro-sync/internal/state"
	"github.com/aaronhurt/vagaro-sync/internal/storage"
	"github.com/aaronhurt/vagaro-sync/internal/vagaro"
)

type fakeAuthStore struct {
	bundle storage.AuthBundle
}

func (s fakeAuthStore) Load(context.Context) (storage.AuthBundle, error) {
	return s.bundle, nil
}

func (s fakeAuthStore) Save(context.Context, storage.AuthBundle) error {
	return nil
}

func (s fakeAuthStore) Delete(context.Context) error {
	return nil
}

type fakeCalendarAdapter struct {
	calendarName    string
	calendarCreated bool
	existingEvents  map[string]bool
	upserts         []string
	deletes         []string
}

func (a *fakeCalendarAdapter) EnsureCalendar(_ context.Context, calendarName string) (bool, error) {
	a.calendarName = calendarName
	return a.calendarCreated, nil
}

func (a *fakeCalendarAdapter) HasEvent(_ context.Context, _ string, eventURL string) (bool, error) {
	return a.existingEvents[eventURL], nil
}

func (a *fakeCalendarAdapter) UpsertEvent(_ context.Context, _ string, event calendar.Event) (string, error) {
	a.upserts = append(a.upserts, event.URL)
	return event.URL, nil
}

func (a *fakeCalendarAdapter) DeleteEvent(_ context.Context, _ string, eventURL string) error {
	a.deletes = append(a.deletes, eventURL)
	return nil
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}

	os.Stdout = writer
	defer func() {
		os.Stdout = originalStdout
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("io.ReadAll() error = %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("reader.Close() error = %v", err)
	}

	return string(output)
}

func TestCommandRunSynchronizesAppointments(t *testing.T) {
	appointments := []vagaro.Appointment{
		{
			AppointmentID: "apt-1",
			SourceHash:    "hash-1",
			Title:         "Haircut",
			StartTimeUTC:  time.Date(2026, time.March, 18, 15, 0, 0, 0, time.UTC),
			EndTimeUTC:    time.Date(2026, time.March, 18, 16, 0, 0, 0, time.UTC),
		},
		{
			AppointmentID: "apt-2",
			SourceHash:    "hash-2-new",
			Title:         "Massage",
			StartTimeUTC:  time.Date(2026, time.March, 19, 15, 0, 0, 0, time.UTC),
			EndTimeUTC:    time.Date(2026, time.March, 19, 16, 0, 0, 0, time.UTC),
		},
	}

	currentState := state.SyncState{
		Appointments: map[string]state.AppointmentState{
			"apt-2": {
				EventID:    "vagaro-sync://appointment/apt-2",
				SourceHash: "hash-2-old",
			},
			"apt-3": {
				EventID:    "vagaro-sync://appointment/apt-3",
				SourceHash: "hash-3",
			},
		},
	}

	adapter := &fakeCalendarAdapter{
		existingEvents: map[string]bool{
			"vagaro-sync://appointment/apt-2": true,
			"vagaro-sync://appointment/apt-3": true,
		},
	}
	stateStore := state.NewFileStore(t.TempDir() + "/state.json")
	if err := stateStore.Save(currentState); err != nil {
		t.Fatalf("stateStore.Save() error = %v", err)
	}

	var gotPageSize int
	origFetchUpcomingAppointments := fetchUpcomingAppointments
	origNewCalendarAdapter := newCalendarAdapter
	t.Cleanup(func() {
		fetchUpcomingAppointments = origFetchUpcomingAppointments
		newCalendarAdapter = origNewCalendarAdapter
	})

	fetchUpcomingAppointments = func(
		_ context.Context,
		_ storage.AuthBundle,
		pageSize int,
	) ([]vagaro.Appointment, error) {
		gotPageSize = pageSize
		return appointments, nil
	}
	newCalendarAdapter = func() calendar.Adapter {
		return adapter
	}

	cmd := &Command{
		AuthStore:  fakeAuthStore{bundle: storage.AuthBundle{SUToken: "token"}},
		StateStore: stateStore,
	}

	output := captureStdout(t, func() {
		if err := cmd.Run(context.Background(), []string{"-page-size=10"}); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	if gotPageSize != 10 {
		t.Fatalf("pageSize = %d, want 10", gotPageSize)
	}
	if adapter.calendarName != "Vagaro Appointments" {
		t.Fatalf("calendarName = %q", adapter.calendarName)
	}
	if len(adapter.upserts) != 2 {
		t.Fatalf("len(upserts) = %d, want 2", len(adapter.upserts))
	}
	if len(adapter.deletes) != 1 || adapter.deletes[0] != "vagaro-sync://appointment/apt-3" {
		t.Fatalf("deletes = %v", adapter.deletes)
	}
	savedState, err := stateStore.Load()
	if err != nil {
		t.Fatalf("stateStore.Load() error = %v", err)
	}
	if len(savedState.Appointments) != 2 {
		t.Fatalf("saved appointments = %d, want 2", len(savedState.Appointments))
	}
	if output != "synced 2 appointments: 1 created, 1 synced, 1 deleted\n" {
		t.Fatalf("output = %q", output)
	}
}

func TestCommandRunRecreatesAppointmentsWhenCalendarWasDeleted(t *testing.T) {
	appointments := []vagaro.Appointment{
		{
			AppointmentID: "apt-1",
			SourceHash:    "hash-1",
			Title:         "Haircut",
			StartTimeUTC:  time.Date(2026, time.March, 18, 15, 0, 0, 0, time.UTC),
			EndTimeUTC:    time.Date(2026, time.March, 18, 16, 0, 0, 0, time.UTC),
		},
	}

	currentState := state.SyncState{
		Appointments: map[string]state.AppointmentState{
			"apt-1": {
				EventID:    "vagaro-sync://appointment/apt-1",
				SourceHash: "hash-1",
			},
		},
	}

	adapter := &fakeCalendarAdapter{calendarCreated: true}
	stateStore := state.NewFileStore(t.TempDir() + "/state.json")
	if err := stateStore.Save(currentState); err != nil {
		t.Fatalf("stateStore.Save() error = %v", err)
	}

	origFetchUpcomingAppointments := fetchUpcomingAppointments
	origNewCalendarAdapter := newCalendarAdapter
	t.Cleanup(func() {
		fetchUpcomingAppointments = origFetchUpcomingAppointments
		newCalendarAdapter = origNewCalendarAdapter
	})

	fetchUpcomingAppointments = func(
		_ context.Context,
		_ storage.AuthBundle,
		_ int,
	) ([]vagaro.Appointment, error) {
		return appointments, nil
	}
	newCalendarAdapter = func() calendar.Adapter {
		return adapter
	}

	cmd := &Command{
		AuthStore:  fakeAuthStore{bundle: storage.AuthBundle{SUToken: "token"}},
		StateStore: stateStore,
	}

	output := captureStdout(t, func() {
		if err := cmd.Run(context.Background(), nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	if len(adapter.upserts) != 1 || adapter.upserts[0] != "vagaro-sync://appointment/apt-1" {
		t.Fatalf("upserts = %v", adapter.upserts)
	}
	if len(adapter.deletes) != 0 {
		t.Fatalf("deletes = %v, want none", adapter.deletes)
	}
	if strings.TrimSpace(output) != "synced 1 appointments: 1 created, 0 synced, 0 deleted" {
		t.Fatalf("output = %q", output)
	}
}

func TestCommandRunRecreatesAppointmentWhenEventWasDeleted(t *testing.T) {
	appointments := []vagaro.Appointment{
		{
			AppointmentID: "apt-1",
			SourceHash:    "hash-1",
			Title:         "Haircut",
			StartTimeUTC:  time.Date(2026, time.March, 18, 15, 0, 0, 0, time.UTC),
			EndTimeUTC:    time.Date(2026, time.March, 18, 16, 0, 0, 0, time.UTC),
		},
	}

	currentState := state.SyncState{
		Appointments: map[string]state.AppointmentState{
			"apt-1": {
				EventID:    "vagaro-sync://appointment/apt-1",
				SourceHash: "hash-1",
			},
		},
	}

	adapter := &fakeCalendarAdapter{
		existingEvents: map[string]bool{
			"vagaro-sync://appointment/apt-1": false,
		},
	}
	stateStore := state.NewFileStore(t.TempDir() + "/state.json")
	if err := stateStore.Save(currentState); err != nil {
		t.Fatalf("stateStore.Save() error = %v", err)
	}

	origFetchUpcomingAppointments := fetchUpcomingAppointments
	origNewCalendarAdapter := newCalendarAdapter
	t.Cleanup(func() {
		fetchUpcomingAppointments = origFetchUpcomingAppointments
		newCalendarAdapter = origNewCalendarAdapter
	})

	fetchUpcomingAppointments = func(
		_ context.Context,
		_ storage.AuthBundle,
		_ int,
	) ([]vagaro.Appointment, error) {
		return appointments, nil
	}
	newCalendarAdapter = func() calendar.Adapter {
		return adapter
	}

	cmd := &Command{
		AuthStore:  fakeAuthStore{bundle: storage.AuthBundle{SUToken: "token"}},
		StateStore: stateStore,
	}

	if err := cmd.Run(context.Background(), nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(adapter.upserts) != 1 || adapter.upserts[0] != "vagaro-sync://appointment/apt-1" {
		t.Fatalf("upserts = %v", adapter.upserts)
	}
}

func TestCommandRunReappliesExistingAppointmentWhenSourceIsUnchanged(t *testing.T) {
	appointments := []vagaro.Appointment{
		{
			AppointmentID: "apt-1",
			SourceHash:    "hash-1",
			Title:         "Haircut",
			StartTimeUTC:  time.Date(2026, time.March, 18, 15, 0, 0, 0, time.UTC),
			EndTimeUTC:    time.Date(2026, time.March, 18, 16, 0, 0, 0, time.UTC),
		},
	}

	currentState := state.SyncState{
		Appointments: map[string]state.AppointmentState{
			"apt-1": {
				EventID:    "vagaro-sync://appointment/apt-1",
				SourceHash: "hash-1",
			},
		},
	}

	adapter := &fakeCalendarAdapter{
		existingEvents: map[string]bool{
			"vagaro-sync://appointment/apt-1": true,
		},
	}
	stateStore := state.NewFileStore(t.TempDir() + "/state.json")
	if err := stateStore.Save(currentState); err != nil {
		t.Fatalf("stateStore.Save() error = %v", err)
	}

	origFetchUpcomingAppointments := fetchUpcomingAppointments
	origNewCalendarAdapter := newCalendarAdapter
	t.Cleanup(func() {
		fetchUpcomingAppointments = origFetchUpcomingAppointments
		newCalendarAdapter = origNewCalendarAdapter
	})

	fetchUpcomingAppointments = func(
		_ context.Context,
		_ storage.AuthBundle,
		_ int,
	) ([]vagaro.Appointment, error) {
		return appointments, nil
	}
	newCalendarAdapter = func() calendar.Adapter {
		return adapter
	}

	cmd := &Command{
		AuthStore:  fakeAuthStore{bundle: storage.AuthBundle{SUToken: "token"}},
		StateStore: stateStore,
	}

	if err := cmd.Run(context.Background(), nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(adapter.upserts) != 1 || adapter.upserts[0] != "vagaro-sync://appointment/apt-1" {
		t.Fatalf("upserts = %v", adapter.upserts)
	}
	if len(adapter.deletes) != 0 {
		t.Fatalf("deletes = %v, want none", adapter.deletes)
	}
}
