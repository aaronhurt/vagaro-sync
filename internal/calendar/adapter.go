// Package calendar synchronizes appointment state into Calendar.app via JXA.
package calendar

import (
	"context"
	_ "embed"
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
	Matches  bool   `json:"matches,omitempty"`
	EventURL string `json:"event_url,omitempty"`
}

// EventStatus reports whether a managed event exists and still matches the expected fields.
type EventStatus struct {
	Exists  bool
	Matches bool
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

// InspectEvent reports whether the managed event exists and still matches the expected fields.
func (a *JXAAdapter) InspectEvent(ctx context.Context, calendarName string, event Event) (EventStatus, error) {
	result, err := a.execute(ctx, scriptInput{
		Action:       "inspect_event",
		CalendarName: calendarName,
		Event:        &event,
	})
	if err != nil {
		return EventStatus{}, err
	}

	return EventStatus{
		Exists:  result.Exists,
		Matches: result.Matches,
	}, nil
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

	// #nosec G204 -- jxaScript is a build-time embedded asset, not user input.
	cmd := exec.CommandContext(ctx, "osascript", "-l", "JavaScript", "-e", jxaScript)
	cmd.Env = append(os.Environ(), "VAGARO_SYNC_INPUT="+string(payload))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run Calendar.app JXA: %w: %s", err, string(output))
	}

	return output, nil
}

//go:embed adapter.jxa.js
var jxaScript string
