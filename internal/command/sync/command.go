// Package synccommand implements the sync CLI command.
package synccommand

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/aaronhurt/vagaro-sync/internal/calendar"
	"github.com/aaronhurt/vagaro-sync/internal/state"
	"github.com/aaronhurt/vagaro-sync/internal/storage"
	"github.com/aaronhurt/vagaro-sync/internal/syncer"
	"github.com/aaronhurt/vagaro-sync/internal/vagaro"
)

const calendarName = "Vagaro Appointments"

var fetchUpcomingAppointments = func(
	ctx context.Context,
	bundle storage.AuthBundle,
	pageSize int,
) ([]vagaro.Appointment, error) {
	client, err := vagaro.NewClient(bundle)
	if err != nil {
		return nil, err
	}

	return client.FetchUpcomingAppointments(ctx, pageSize)
}

var newCalendarAdapter = func() calendar.Adapter {
	return calendar.NewJXAAdapter()
}

// Command runs the sync flow.
type Command struct {
	AuthStore  storage.AuthStore
	StateStore *state.FileStore
}

// Run executes the sync command.
func (c *Command) Run(ctx context.Context, args []string) error {
	cmd := flag.NewFlagSet("sync", flag.ContinueOnError)
	cmd.SetOutput(os.Stderr)

	pageSize := cmd.Int("page-size", 24, "appointments page size for Vagaro requests")
	if err := cmd.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}

		return err
	}

	bundle, err := c.AuthStore.Load(ctx)
	if err != nil {
		return fmt.Errorf("load authentication bundle: %w", err)
	}

	appointments, err := fetchUpcomingAppointments(ctx, bundle, *pageSize)
	if err != nil {
		return err
	}

	currentState, err := c.StateStore.Load()
	if err != nil {
		return err
	}

	adapter := newCalendarAdapter()

	calendarCreated, err := adapter.EnsureCalendar(ctx, calendarName)
	if err != nil {
		return err
	}
	if calendarCreated {
		currentState = state.SyncState{Appointments: map[string]state.AppointmentState{}}
	} else {
		currentState, err = pruneMissingEvents(ctx, adapter, currentState)
		if err != nil {
			return err
		}
	}

	plan := syncer.BuildPlan(appointments, currentState)

	for _, event := range plan.Creates {
		if _, err := adapter.UpsertEvent(ctx, calendarName, event); err != nil {
			return err
		}
	}

	for _, event := range plan.Updates {
		if _, err := adapter.UpsertEvent(ctx, calendarName, event); err != nil {
			return err
		}
	}

	for _, deletion := range plan.Deletes {
		if err := adapter.DeleteEvent(ctx, calendarName, deletion.EventURL); err != nil {
			return err
		}
	}

	if err := c.StateStore.Save(plan.NextState); err != nil {
		return err
	}

	_, err = fmt.Fprintf(
		os.Stdout,
		"synced %d appointments: %d created, %d updated, %d deleted\n",
		len(appointments),
		len(plan.Creates),
		len(plan.Updates),
		len(plan.Deletes),
	)
	return err
}

func pruneMissingEvents(
	ctx context.Context,
	adapter calendar.Adapter,
	currentState state.SyncState,
) (state.SyncState, error) {
	if len(currentState.Appointments) == 0 {
		return currentState, nil
	}

	pruned := state.SyncState{
		Appointments: make(map[string]state.AppointmentState, len(currentState.Appointments)),
	}

	for appointmentID, appointmentState := range currentState.Appointments {
		if appointmentState.EventID == "" {
			continue
		}

		exists, err := adapter.HasEvent(ctx, calendarName, appointmentState.EventID)
		if err != nil {
			return state.SyncState{}, err
		}
		if !exists {
			continue
		}

		pruned.Appointments[appointmentID] = appointmentState
	}

	return pruned, nil
}
