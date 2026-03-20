package calendar

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	lastInput scriptInput
	output    []byte
	err       error
	called    bool
}

func (r *fakeRunner) Run(_ context.Context, input scriptInput) ([]byte, error) {
	r.lastInput = input
	r.called = true
	return r.output, r.err
}

func TestUpsertEventPassesExpectedInput(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		output: []byte(`{"ok":true,"event_url":"vagaro-sync://appointment/apt-1"}`),
	}
	adapter := &JXAAdapter{runner: runner}

	eventURL, err := adapter.UpsertEvent(context.Background(), "Vagaro Appointments", Event{
		URL:          "vagaro-sync://appointment/apt-1",
		Title:        "Haircut @ Salon One",
		StartTimeUTC: time.Date(2026, time.March, 18, 15, 0, 0, 0, time.UTC),
		EndTimeUTC:   time.Date(2026, time.March, 18, 16, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("UpsertEvent() error = %v", err)
	}

	if eventURL != "vagaro-sync://appointment/apt-1" {
		t.Fatalf("eventURL = %q", eventURL)
	}
	if runner.lastInput.Action != "upsert_event" {
		t.Fatalf("Action = %q", runner.lastInput.Action)
	}
	if runner.lastInput.CalendarName != "Vagaro Appointments" {
		t.Fatalf("CalendarName = %q", runner.lastInput.CalendarName)
	}
	if runner.lastInput.Event == nil || runner.lastInput.Event.URL == "" {
		t.Fatalf("Event input missing URL: %+v", runner.lastInput.Event)
	}
	if !runner.called {
		t.Fatalf("expected runner to be invoked")
	}
}

func TestEnsureCalendarAcceptsSuccessResponse(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		output: []byte(`{"ok":true,"created":true}`),
	}
	adapter := &JXAAdapter{runner: runner}

	created, err := adapter.EnsureCalendar(context.Background(), "Vagaro Appointments")
	if err != nil {
		t.Fatalf("EnsureCalendar() error = %v", err)
	}

	if runner.lastInput.Action != "ensure_calendar" {
		t.Fatalf("Action = %q", runner.lastInput.Action)
	}
	if !created {
		t.Fatal("expected created=true")
	}
}

func TestInspectEventPassesExpectedInput(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		output: []byte(`{"ok":true,"exists":true,"matches":true}`),
	}
	adapter := &JXAAdapter{runner: runner}

	status, err := adapter.InspectEvent(context.Background(), "Vagaro Appointments", Event{
		URL:          "vagaro-sync://appointment/apt-1",
		Title:        "Haircut @ Salon One",
		StartTimeUTC: time.Date(2026, time.March, 18, 15, 0, 0, 0, time.UTC),
		EndTimeUTC:   time.Date(2026, time.March, 18, 16, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("InspectEvent() error = %v", err)
	}

	if !status.Exists {
		t.Fatal("expected exists=true")
	}
	if !status.Matches {
		t.Fatal("expected matches=true")
	}
	if runner.lastInput.Action != "inspect_event" {
		t.Fatalf("Action = %q", runner.lastInput.Action)
	}
	if runner.lastInput.Event == nil || runner.lastInput.Event.URL != "vagaro-sync://appointment/apt-1" {
		t.Fatalf("Event input missing URL: %+v", runner.lastInput.Event)
	}
}

func TestJXAScriptExpandsEventRangeBeforeApplyingFinalTimes(t *testing.T) {
	t.Parallel()

	expectedSnippets := []string{
		"var desiredStart = new Date(payload.start_time_utc);",
		"var desiredEnd = new Date(payload.end_time_utc);",
		"var currentStart = event.startDate();",
		"var currentEnd = event.endDate();",
		"var expandedStart = currentStart < desiredStart ? currentStart : desiredStart;",
		"var expandedEnd = currentEnd > desiredEnd ? currentEnd : desiredEnd;",
		"event.startDate = expandedStart;",
		"event.endDate = expandedEnd;",
		"event.startDate = desiredStart;",
		"event.endDate = desiredEnd;",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(jxaScript, snippet) {
			t.Fatalf("jxaScript missing snippet %q", snippet)
		}
	}
}

func TestJXAScriptComparesManagedEventFields(t *testing.T) {
	t.Parallel()

	expectedSnippets := []string{
		"function eventMatches(event, payload) {",
		"return event.summary() === payload.title &&",
		"event.location() === (payload.location || '') &&",
		"event.description() === (payload.notes || '') &&",
		"event.url() === payload.url &&",
		"event.startDate().getTime() === new Date(payload.start_time_utc).getTime() &&",
		"event.endDate().getTime() === new Date(payload.end_time_utc).getTime();",
		"if (input.action === 'inspect_event') {",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(jxaScript, snippet) {
			t.Fatalf("jxaScript missing snippet %q", snippet)
		}
	}
}
