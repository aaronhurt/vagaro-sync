// Package syncer plans calendar reconciliation from normalized Vagaro appointments.
package syncer

import (
	"net/url"
	"time"

	"github.com/aaronhurt/vagaro-sync/internal/calendar"
	"github.com/aaronhurt/vagaro-sync/internal/state"
	"github.com/aaronhurt/vagaro-sync/internal/vagaro"
)

// DeleteOperation describes a calendar event that should be removed.
type DeleteOperation struct {
	AppointmentID string
	EventURL      string
}

// Plan contains the create, update, and delete operations for a sync pass.
type Plan struct {
	Creates   []calendar.Event
	Updates   []calendar.Event
	Unchanged []calendar.Event
	Deletes   []DeleteOperation
	NextState state.SyncState
}

// BuildPlan compares appointments against current state and returns a reconciliation plan.
func BuildPlan(appointments []vagaro.Appointment, current state.SyncState) Plan {
	if current.Appointments == nil {
		current.Appointments = map[string]state.AppointmentState{}
	}

	nextState := state.SyncState{
		Appointments: make(map[string]state.AppointmentState, len(appointments)),
	}

	seen := make(map[string]struct{}, len(appointments))
	plan := Plan{
		Creates:   make([]calendar.Event, 0),
		Updates:   make([]calendar.Event, 0),
		Unchanged: make([]calendar.Event, 0),
		Deletes:   make([]DeleteOperation, 0),
		NextState: nextState,
	}

	now := time.Now().UTC()
	for _, appointment := range appointments {
		event := calendarEventForAppointment(appointment)
		nextState.Appointments[appointment.AppointmentID] = state.AppointmentState{
			EventID:    event.URL,
			SourceHash: appointment.SourceHash,
			UpdatedAt:  now,
		}
		seen[appointment.AppointmentID] = struct{}{}

		previous, ok := current.Appointments[appointment.AppointmentID]
		if !ok {
			plan.Creates = append(plan.Creates, event)
			continue
		}

		if previous.SourceHash == appointment.SourceHash {
			plan.Unchanged = append(plan.Unchanged, event)
			continue
		}

		plan.Updates = append(plan.Updates, event)
	}

	for sourceID, previous := range current.Appointments {
		if _, ok := seen[sourceID]; ok {
			continue
		}

		eventURL := previous.EventID
		if eventURL == "" {
			eventURL = eventURLForAppointment(sourceID)
		}

		plan.Deletes = append(plan.Deletes, DeleteOperation{
			AppointmentID: sourceID,
			EventURL:      eventURL,
		})
	}

	return plan
}

func calendarEventForAppointment(appointment vagaro.Appointment) calendar.Event {
	eventURL := eventURLForAppointment(appointment.AppointmentID)
	notes := appointment.Notes
	if notes != "" {
		notes += "\n\n"
	}
	notes += "Managed by vagaro-sync\nAppointment ID: " + appointment.AppointmentID

	return calendar.Event{
		URL:          eventURL,
		Title:        appointment.Title,
		Location:     appointment.Location,
		Notes:        notes,
		StartTimeUTC: appointment.StartTimeUTC,
		EndTimeUTC:   appointment.EndTimeUTC,
	}
}

func eventURLForAppointment(appointmentID string) string {
	return "vagaro-sync://appointment/" + url.PathEscape(appointmentID)
}
