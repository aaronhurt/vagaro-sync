// Package synccommand implements the sync CLI command.
package synccommand

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/aaronhurt/vagaro-sync/internal/calendar"
	"github.com/aaronhurt/vagaro-sync/internal/state"
	"github.com/aaronhurt/vagaro-sync/internal/storage"
	"github.com/aaronhurt/vagaro-sync/internal/syncer"
	"github.com/aaronhurt/vagaro-sync/internal/vagaro"
)

const calendarName = "Vagaro Appointments"

type authStore interface {
	Load(context.Context) (storage.AuthBundle, error)
}

type appointmentFetcher interface {
	FetchUpcomingAppointments(context.Context, storage.AuthBundle, int) ([]vagaro.Appointment, error)
}

type calendarAdapter interface {
	EnsureCalendar(context.Context, string) (bool, error)
	InspectEvent(context.Context, string, calendar.Event) (calendar.EventStatus, error)
	UpsertEvent(context.Context, string, calendar.Event) (string, error)
	DeleteEvent(context.Context, string, string) error
}

type calendarFactory interface {
	New() calendarAdapter
}

type vagaroFetcher struct{}

func (vagaroFetcher) FetchUpcomingAppointments(
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

type jxaCalendarFactory struct{}

func (jxaCalendarFactory) New() calendarAdapter {
	return calendar.NewJXAAdapter()
}

// Command runs the sync flow.
type Command struct {
	authStore          authStore
	stateStore         *state.FileStore
	appointmentFetcher appointmentFetcher
	calendarFactory    calendarFactory
	now                func() time.Time
}

// NewCommand constructs the sync command.
func NewCommand(store *storage.KeychainStore, stateStore *state.FileStore) *Command {
	return &Command{
		authStore:          store,
		stateStore:         stateStore,
		appointmentFetcher: vagaroFetcher{},
		calendarFactory:    jxaCalendarFactory{},
		now:                time.Now,
	}
}

// Run executes the sync command.
func (c *Command) Run(ctx context.Context, args []string) error {
	if c.appointmentFetcher == nil {
		c.appointmentFetcher = vagaroFetcher{}
	}
	if c.calendarFactory == nil {
		c.calendarFactory = jxaCalendarFactory{}
	}
	if c.now == nil {
		c.now = time.Now
	}

	cmd := flag.NewFlagSet("sync", flag.ContinueOnError)
	cmd.SetOutput(os.Stderr)

	pageSize := cmd.Int("page-size", 24, "appointments page size for Vagaro requests")
	if err := cmd.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}

		return err
	}

	bundle, err := c.authStore.Load(ctx)
	if err != nil {
		return fmt.Errorf("load authentication bundle: %w", err)
	}
	if err := vagaro.ValidateAuthBundle(bundle, c.now().UTC()); err != nil {
		return wrapAuthenticationError(err)
	}

	appointments, err := c.appointmentFetcher.FetchUpcomingAppointments(ctx, bundle, *pageSize)
	if err != nil {
		return wrapAuthenticationError(err)
	}

	currentState, loadStatus, err := c.stateStore.Load()
	if err != nil {
		return err
	}
	if loadStatus.Corrupted {
		if _, err := fmt.Fprintln(
			os.Stderr,
			"warning: sync state was corrupted and has been reset; sync will continue, but stale managed Calendar events may remain",
		); err != nil {
			return err
		}
	}

	adapter := c.calendarFactory.New()

	calendarCreated, err := adapter.EnsureCalendar(ctx, calendarName)
	if err != nil {
		return err
	}
	if calendarCreated {
		currentState = state.SyncState{Appointments: map[string]state.AppointmentState{}}
	}

	plan := syncer.BuildPlan(appointments, currentState)
	plan, err = reclassifyRetainedEvents(ctx, adapter, plan)
	if err != nil {
		return err
	}

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

	if err := c.stateStore.Save(plan.NextState); err != nil {
		return err
	}

	_, err = fmt.Fprintf(
		os.Stdout,
		"synced %d appointments: %d created, %d updated, %d unchanged, %d deleted\n",
		len(appointments),
		len(plan.Creates),
		len(plan.Updates),
		len(plan.Unchanged),
		len(plan.Deletes),
	)
	return err
}

func wrapAuthenticationError(err error) error {
	if !vagaro.IsAuthenticationInvalid(err) {
		return err
	}

	return fmt.Errorf("authentication invalid: %w; run `vagaro-sync auth-login`", err)
}

func reclassifyRetainedEvents(ctx context.Context, adapter calendarAdapter, plan syncer.Plan) (syncer.Plan, error) {
	if len(plan.Unchanged) == 0 {
		return plan, nil
	}

	stable := make([]calendar.Event, 0, len(plan.Unchanged))
	for _, event := range plan.Unchanged {
		status, err := adapter.InspectEvent(ctx, calendarName, event)
		if err != nil {
			return syncer.Plan{}, err
		}
		if status.Exists && status.Matches {
			stable = append(stable, event)
			continue
		}

		plan.Updates = append(plan.Updates, event)
	}

	plan.Unchanged = stable
	return plan, nil
}
