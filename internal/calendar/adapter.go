// Package calendar synchronizes appointment state into Calendar.app via JXA.
package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Event describes the Calendar.app event representation of a Vagaro appointment.
type Event struct {
	URL          string    `json:"url"`
	Title        string    `json:"title"`
	Location     string    `json:"location,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	StartTimeUTC time.Time `json:"start_time_utc"`
	EndTimeUTC   time.Time `json:"end_time_utc"`
}

// Adapter reconciles events in a target calendar.
type Adapter interface {
	EnsureCalendar(context.Context, string) (bool, error)
	HasEvent(context.Context, string, string) (bool, error)
	UpsertEvent(context.Context, string, Event) (string, error)
	DeleteEvent(context.Context, string, string) error
}

type scriptRunner interface {
	Run(context.Context, scriptInput) ([]byte, error)
}

// JXAAdapter implements Calendar.app operations through osascript JavaScript automation.
type JXAAdapter struct {
	runner scriptRunner
}

type scriptInput struct {
	Action       string `json:"action"`
	CalendarName string `json:"calendar_name"`
	Event        *Event `json:"event,omitempty"`
	EventURL     string `json:"event_url,omitempty"`
}

type scriptResult struct {
	OK       bool   `json:"ok"`
	Created  bool   `json:"created,omitempty"`
	Exists   bool   `json:"exists,omitempty"`
	EventURL string `json:"event_url,omitempty"`
}

// NewJXAAdapter returns a Calendar.app adapter backed by JXA.
func NewJXAAdapter() *JXAAdapter {
	return &JXAAdapter{
		runner: osascriptRunner{},
	}
}

// EnsureCalendar creates the named calendar when it does not already exist.
func (a *JXAAdapter) EnsureCalendar(ctx context.Context, calendarName string) (bool, error) {
	result, err := a.execute(ctx, scriptInput{
		Action:       "ensure_calendar",
		CalendarName: calendarName,
	})
	if err != nil {
		return false, err
	}

	return result.Created, nil
}

// UpsertEvent creates or updates an event in the named calendar and returns its URL.
func (a *JXAAdapter) UpsertEvent(ctx context.Context, calendarName string, event Event) (string, error) {
	result, err := a.execute(ctx, scriptInput{
		Action:       "upsert_event",
		CalendarName: calendarName,
		Event:        &event,
	})
	if err != nil {
		return "", err
	}

	return result.EventURL, nil
}

// HasEvent reports whether the event with the provided URL exists in the named calendar.
func (a *JXAAdapter) HasEvent(ctx context.Context, calendarName string, eventURL string) (bool, error) {
	result, err := a.execute(ctx, scriptInput{
		Action:       "has_event",
		CalendarName: calendarName,
		EventURL:     eventURL,
	})
	if err != nil {
		return false, err
	}

	return result.Exists, nil
}

// DeleteEvent removes the event with the provided URL from the named calendar.
func (a *JXAAdapter) DeleteEvent(ctx context.Context, calendarName string, eventURL string) error {
	_, err := a.execute(ctx, scriptInput{
		Action:       "delete_event",
		CalendarName: calendarName,
		EventURL:     eventURL,
	})
	return err
}

func (a *JXAAdapter) execute(ctx context.Context, input scriptInput) (scriptResult, error) {
	output, err := a.runner.Run(ctx, input)
	if err != nil {
		return scriptResult{}, err
	}

	var result scriptResult
	if err := json.Unmarshal(output, &result); err != nil {
		return scriptResult{}, fmt.Errorf("decode Calendar.app response: %w", err)
	}

	if !result.OK {
		return scriptResult{}, fmt.Errorf("calendar command did not report success")
	}

	return result, nil
}

type osascriptRunner struct{}

func (osascriptRunner) Run(ctx context.Context, input scriptInput) ([]byte, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode Calendar.app input: %w", err)
	}

	cmd := exec.CommandContext(ctx, "osascript", "-l", "JavaScript", "-e", jxaScript)
	cmd.Env = append(os.Environ(), "VAGARO_SYNC_INPUT="+string(payload))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run Calendar.app JXA: %w: %s", err, string(output))
	}

	return output, nil
}

const jxaScript = `
ObjC.import('stdlib');

function findCalendar(app, name) {
  var matches = app.calendars.whose({name: name})();
  if (matches.length > 0) {
    return {calendar: matches[0], created: false};
  }

  var calendar = app.Calendar({name: name});
  app.calendars.push(calendar);
  return {calendar: app.calendars.whose({name: name})()[0], created: true};
}

function findEvent(calendar, eventURL) {
  var matches = calendar.events.whose({url: eventURL})();
  if (matches.length > 0) {
    return matches[0];
  }

  return null;
}

function applyEventFields(event, payload) {
  event.summary = payload.title;
  event.startDate = new Date(payload.start_time_utc);
  event.endDate = new Date(payload.end_time_utc);
  event.location = payload.location || '';
  event.description = payload.notes || '';
  event.url = payload.url;
}

function run() {
  var input = JSON.parse(ObjC.unwrap($.getenv('VAGARO_SYNC_INPUT')));
  var calendarApp = Application('Calendar');
  calendarApp.includeStandardAdditions = true;
  var calendarResult = findCalendar(calendarApp, input.calendar_name);
  var calendar = calendarResult.calendar;

  if (input.action === 'ensure_calendar') {
    return JSON.stringify({ok: true, created: calendarResult.created});
  }

  if (input.action === 'upsert_event') {
    var existing = findEvent(calendar, input.event.url);
    if (existing === null) {
      var created = calendarApp.Event({
        summary: input.event.title,
        startDate: new Date(input.event.start_time_utc),
        endDate: new Date(input.event.end_time_utc),
        location: input.event.location || '',
        description: input.event.notes || '',
        url: input.event.url
      });
      calendar.events.push(created);
    } else {
      applyEventFields(existing, input.event);
    }

    return JSON.stringify({ok: true, event_url: input.event.url});
  }

  if (input.action === 'has_event') {
    return JSON.stringify({ok: true, exists: findEvent(calendar, input.event_url) !== null});
  }

  if (input.action === 'delete_event') {
    var eventToDelete = findEvent(calendar, input.event_url);
    if (eventToDelete !== null) {
      eventToDelete.delete();
    }

    return JSON.stringify({ok: true});
  }

  throw new Error('unsupported action: ' + input.action);
}
`
