package synccommand

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

type fakeAppointmentFetcher struct {
	pageSize int
	result   []vagaro.Appointment
	err      error
}

func (f *fakeAppointmentFetcher) FetchUpcomingAppointments(
	_ context.Context,
	_ storage.AuthBundle,
	pageSize int,
) ([]vagaro.Appointment, error) {
	f.pageSize = pageSize
	return f.result, f.err
}

type fakeCalendarAdapter struct {
	calendarName    string
	calendarCreated bool
	eventStatus     map[string]calendar.EventStatus
	upserts         []string
	deletes         []string
}

func (a *fakeCalendarAdapter) EnsureCalendar(_ context.Context, calendarName string) (bool, error) {
	a.calendarName = calendarName
	return a.calendarCreated, nil
}

func (a *fakeCalendarAdapter) InspectEvent(
	_ context.Context,
	_ string,
	event calendar.Event,
) (calendar.EventStatus, error) {
	if a.eventStatus == nil {
		return calendar.EventStatus{Exists: true, Matches: true}, nil
	}

	status, ok := a.eventStatus[event.URL]
	if !ok {
		return calendar.EventStatus{Exists: true, Matches: true}, nil
	}

	return status, nil
}

func (a *fakeCalendarAdapter) UpsertEvent(_ context.Context, _ string, event calendar.Event) (string, error) {
	a.upserts = append(a.upserts, event.URL)
	return event.URL, nil
}

func (a *fakeCalendarAdapter) DeleteEvent(_ context.Context, _ string, eventURL string) error {
	a.deletes = append(a.deletes, eventURL)
	return nil
}

type fakeCalendarFactory struct {
	adapter calendarAdapter
}

func (f fakeCalendarFactory) New() calendarAdapter {
	return f.adapter
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

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	originalStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}

	os.Stderr = writer
	defer func() {
		os.Stderr = originalStderr
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
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)
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
		eventStatus: map[string]calendar.EventStatus{
			"vagaro-sync://appointment/apt-2": {Exists: true, Matches: true},
			"vagaro-sync://appointment/apt-3": {Exists: true, Matches: true},
		},
	}
	stateStore := state.NewFileStore(t.TempDir() + "/state.json")
	if err := stateStore.Save(currentState); err != nil {
		t.Fatalf("stateStore.Save() error = %v", err)
	}

	fetcher := &fakeAppointmentFetcher{result: appointments}
	cmd := &Command{
		authStore:          fakeAuthStore{bundle: storage.AuthBundle{SUToken: testJWT(t, now.Add(5*time.Minute))}},
		stateStore:         stateStore,
		appointmentFetcher: fetcher,
		calendarFactory:    fakeCalendarFactory{adapter: adapter},
		now:                func() time.Time { return now },
	}

	output := captureStdout(t, func() {
		if err := cmd.Run(context.Background(), []string{"-page-size=10"}); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	if fetcher.pageSize != 10 {
		t.Fatalf("pageSize = %d, want 10", fetcher.pageSize)
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
	savedState, status, err := stateStore.Load()
	if err != nil {
		t.Fatalf("stateStore.Load() error = %v", err)
	}
	if status.Corrupted {
		t.Fatal("expected saved state not to be marked corrupted")
	}
	if len(savedState.Appointments) != 2 {
		t.Fatalf("saved appointments = %d, want 2", len(savedState.Appointments))
	}
	if output != "synced 2 appointments: 1 created, 1 updated, 0 unchanged, 1 deleted\n" {
		t.Fatalf("output = %q", output)
	}
}

func TestCommandRunRecreatesAppointmentsWhenCalendarWasDeleted(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)
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

	cmd := &Command{
		authStore:          fakeAuthStore{bundle: storage.AuthBundle{SUToken: testJWT(t, now.Add(5*time.Minute))}},
		stateStore:         stateStore,
		appointmentFetcher: &fakeAppointmentFetcher{result: appointments},
		calendarFactory:    fakeCalendarFactory{adapter: adapter},
		now:                func() time.Time { return now },
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
	if strings.TrimSpace(output) != "synced 1 appointments: 1 created, 0 updated, 0 unchanged, 0 deleted" {
		t.Fatalf("output = %q", output)
	}
}

func TestCommandRunRecreatesAppointmentWhenEventWasDeleted(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)
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

	adapter := &fakeCalendarAdapter{eventStatus: map[string]calendar.EventStatus{
		"vagaro-sync://appointment/apt-1": {Exists: false, Matches: false},
	}}
	stateStore := state.NewFileStore(t.TempDir() + "/state.json")
	if err := stateStore.Save(currentState); err != nil {
		t.Fatalf("stateStore.Save() error = %v", err)
	}

	cmd := &Command{
		authStore:          fakeAuthStore{bundle: storage.AuthBundle{SUToken: testJWT(t, now.Add(5*time.Minute))}},
		stateStore:         stateStore,
		appointmentFetcher: &fakeAppointmentFetcher{result: appointments},
		calendarFactory:    fakeCalendarFactory{adapter: adapter},
		now:                func() time.Time { return now },
	}

	if err := cmd.Run(context.Background(), nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(adapter.upserts) != 1 || adapter.upserts[0] != "vagaro-sync://appointment/apt-1" {
		t.Fatalf("upserts = %v", adapter.upserts)
	}
}

func TestCommandRunSkipsUpsertWhenSourceIsUnchanged(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)
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

	adapter := &fakeCalendarAdapter{eventStatus: map[string]calendar.EventStatus{
		"vagaro-sync://appointment/apt-1": {Exists: true, Matches: true},
	}}
	stateStore := state.NewFileStore(t.TempDir() + "/state.json")
	if err := stateStore.Save(currentState); err != nil {
		t.Fatalf("stateStore.Save() error = %v", err)
	}

	cmd := &Command{
		authStore:          fakeAuthStore{bundle: storage.AuthBundle{SUToken: testJWT(t, now.Add(5*time.Minute))}},
		stateStore:         stateStore,
		appointmentFetcher: &fakeAppointmentFetcher{result: appointments},
		calendarFactory:    fakeCalendarFactory{adapter: adapter},
		now:                func() time.Time { return now },
	}

	output := captureStdout(t, func() {
		if err := cmd.Run(context.Background(), nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	if len(adapter.upserts) != 0 {
		t.Fatalf("upserts = %v, want none", adapter.upserts)
	}
	if len(adapter.deletes) != 0 {
		t.Fatalf("deletes = %v, want none", adapter.deletes)
	}
	if strings.TrimSpace(output) != "synced 1 appointments: 0 created, 0 updated, 1 unchanged, 0 deleted" {
		t.Fatalf("output = %q", output)
	}
}

func TestCommandRunUpdatesDriftedAppointmentWhenCalendarEventChanged(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)
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

	adapter := &fakeCalendarAdapter{eventStatus: map[string]calendar.EventStatus{
		"vagaro-sync://appointment/apt-1": {Exists: true, Matches: false},
	}}
	stateStore := state.NewFileStore(t.TempDir() + "/state.json")
	if err := stateStore.Save(currentState); err != nil {
		t.Fatalf("stateStore.Save() error = %v", err)
	}

	cmd := &Command{
		authStore:          fakeAuthStore{bundle: storage.AuthBundle{SUToken: testJWT(t, now.Add(5*time.Minute))}},
		stateStore:         stateStore,
		appointmentFetcher: &fakeAppointmentFetcher{result: appointments},
		calendarFactory:    fakeCalendarFactory{adapter: adapter},
		now:                func() time.Time { return now },
	}

	output := captureStdout(t, func() {
		if err := cmd.Run(context.Background(), nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	if len(adapter.upserts) != 1 || adapter.upserts[0] != "vagaro-sync://appointment/apt-1" {
		t.Fatalf("upserts = %v", adapter.upserts)
	}
	if strings.TrimSpace(output) != "synced 1 appointments: 0 created, 1 updated, 0 unchanged, 0 deleted" {
		t.Fatalf("output = %q", output)
	}
}

func TestCommandRunReturnsReauthGuidanceForExpiredToken(t *testing.T) {
	t.Parallel()

	cmd := &Command{
		authStore: fakeAuthStore{
			bundle: storage.AuthBundle{SUToken: testJWT(t, time.Date(2026, time.March, 18, 11, 0, 0, 0, time.UTC))},
		},
		stateStore:         state.NewFileStore(t.TempDir() + "/state.json"),
		appointmentFetcher: &fakeAppointmentFetcher{},
		calendarFactory:    fakeCalendarFactory{adapter: &fakeCalendarAdapter{}},
		now:                func() time.Time { return time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC) },
	}

	err := cmd.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected authentication error")
	}
	if !strings.Contains(err.Error(), "auth-login") {
		t.Fatalf("error = %v, want reauth guidance", err)
	}
}

func TestCommandRunReturnsReauthGuidanceForServerAuthFailure(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)
	cmd := &Command{
		authStore: fakeAuthStore{
			bundle: storage.AuthBundle{SUToken: testJWT(t, now.Add(5*time.Minute))},
		},
		stateStore:         state.NewFileStore(t.TempDir() + "/state.json"),
		appointmentFetcher: &fakeAppointmentFetcher{err: fmtAuthError()},
		calendarFactory:    fakeCalendarFactory{adapter: &fakeCalendarAdapter{}},
		now:                func() time.Time { return now },
	}

	err := cmd.Run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected authentication error")
	}
	if !strings.Contains(err.Error(), "auth-login") {
		t.Fatalf("error = %v, want reauth guidance", err)
	}
}

func TestCommandRunWarnsWhenStateWasCorrupted(t *testing.T) {
	now := time.Date(2026, time.March, 18, 12, 0, 0, 0, time.UTC)
	statePath := t.TempDir() + "/state.json"
	if err := os.WriteFile(statePath, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	cmd := &Command{
		authStore:          fakeAuthStore{bundle: storage.AuthBundle{SUToken: testJWT(t, now.Add(5*time.Minute))}},
		stateStore:         state.NewFileStore(statePath),
		appointmentFetcher: &fakeAppointmentFetcher{},
		calendarFactory:    fakeCalendarFactory{adapter: &fakeCalendarAdapter{}},
		now:                func() time.Time { return now },
	}

	stderr := captureStderr(t, func() {
		if err := cmd.Run(context.Background(), nil); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	})

	if !strings.Contains(stderr, "warning: sync state was corrupted and has been reset") {
		t.Fatalf("stderr = %q", stderr)
	}
	if !strings.Contains(stderr, "stale managed Calendar events may remain") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func fmtAuthError() error {
	return fmt.Errorf("fetch appointments: %w", vagaro.ErrAuthenticationInvalid)
}

func testJWT(t *testing.T, exp time.Time) string {
	t.Helper()

	header, err := json.Marshal(map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("json.Marshal(header) error = %v", err)
	}
	payload, err := json.Marshal(map[string]int64{
		"exp": exp.UTC().Unix(),
	})
	if err != nil {
		t.Fatalf("json.Marshal(payload) error = %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(header) +
		"." +
		base64.RawURLEncoding.EncodeToString(payload) +
		".signature"
}
