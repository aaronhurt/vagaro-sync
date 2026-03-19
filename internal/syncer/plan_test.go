package syncer

import (
	"testing"
	"time"

	"github.com/aaronhurt/vagaro-sync/internal/state"
	"github.com/aaronhurt/vagaro-sync/internal/vagaro"
)

func TestBuildPlanProducesCreateUpdateDeleteSets(t *testing.T) {
	t.Parallel()

	appointments := []vagaro.Appointment{
		{
			AppointmentID: "apt-1",
			SourceHash:    "hash-1",
			Title:         "Haircut",
			StartTimeUTC:  time.Date(2026, time.March, 18, 15, 0, 0, 0, time.UTC),
			EndTimeUTC:    time.Date(2026, time.March, 18, 16, 0, 0, 0, time.UTC),
			Location:      "123 Main St",
			Notes:         "Staff: Alex",
		},
		{
			AppointmentID: "apt-2",
			SourceHash:    "hash-2-new",
			Title:         "Massage",
			StartTimeUTC:  time.Date(2026, time.March, 19, 15, 0, 0, 0, time.UTC),
			EndTimeUTC:    time.Date(2026, time.March, 19, 16, 0, 0, 0, time.UTC),
		},
	}

	current := state.SyncState{
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

	plan := BuildPlan(appointments, current)

	if len(plan.Creates) != 1 {
		t.Fatalf("len(Creates) = %d, want 1", len(plan.Creates))
	}
	if len(plan.Updates) != 1 {
		t.Fatalf("len(Updates) = %d, want 1", len(plan.Updates))
	}
	if len(plan.Deletes) != 1 {
		t.Fatalf("len(Deletes) = %d, want 1", len(plan.Deletes))
	}
	if len(plan.NextState.Appointments) != 2 {
		t.Fatalf("len(NextState.Appointments) = %d, want 2", len(plan.NextState.Appointments))
	}
	if plan.Creates[0].URL != "vagaro-sync://appointment/apt-1" {
		t.Fatalf("Creates[0].URL = %q", plan.Creates[0].URL)
	}
	if plan.Deletes[0].EventURL != "vagaro-sync://appointment/apt-3" {
		t.Fatalf("Deletes[0].EventURL = %q", plan.Deletes[0].EventURL)
	}
}

func TestBuildPlanUpdatesExistingAppointmentWhenSourceIsUnchanged(t *testing.T) {
	t.Parallel()

	appointments := []vagaro.Appointment{
		{
			AppointmentID: "apt-1",
			SourceHash:    "hash-1",
			Title:         "Haircut",
			StartTimeUTC:  time.Date(2026, time.March, 18, 15, 0, 0, 0, time.UTC),
			EndTimeUTC:    time.Date(2026, time.March, 18, 16, 0, 0, 0, time.UTC),
		},
	}

	current := state.SyncState{
		Appointments: map[string]state.AppointmentState{
			"apt-1": {
				EventID:    "vagaro-sync://appointment/apt-1",
				SourceHash: "hash-1",
			},
		},
	}

	plan := BuildPlan(appointments, current)

	if len(plan.Creates) != 0 {
		t.Fatalf("len(Creates) = %d, want 0", len(plan.Creates))
	}
	if len(plan.Updates) != 1 {
		t.Fatalf("len(Updates) = %d, want 1", len(plan.Updates))
	}
	if plan.Updates[0].URL != "vagaro-sync://appointment/apt-1" {
		t.Fatalf("Updates[0].URL = %q", plan.Updates[0].URL)
	}
	if len(plan.Deletes) != 0 {
		t.Fatalf("len(Deletes) = %d, want 0", len(plan.Deletes))
	}
}
