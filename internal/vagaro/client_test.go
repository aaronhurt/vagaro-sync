package vagaro

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchUpcomingAppointmentsUsesTypedPayload(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++

		if r.Method != http.MethodPost {
			t.Fatalf("Method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q, want application/json", got)
		}
		if got := r.Header.Get("s_utkn"); got != "token" {
			t.Fatalf("s_utkn = %q, want token", got)
		}

		var req fetchAppointmentsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if req.Device != requestDevice || req.Module != requestModule || req.Version != requestVersion {
			t.Fatalf("request metadata = %+v", req)
		}

		switch req.PageNumber {
		case 1:
			writeJSON(t, w, fetchAppointmentsResponse{
				Status:       http.StatusOK,
				ResponseCode: successResponseCode,
				Message:      "Success",
				Data: []appointmentPayload{
					{
						AppointmentID:            "apt-1",
						BusinessID:               "biz-1",
						BusinessName:             "Salon One",
						ServiceTitle:             "Haircut - 45 mins",
						ServiceProviderFirstName: "Taylor",
						ServiceProviderLastName:  "Smith",
						AppStatusTitle:           "Accepted",
						StartTimeUTC:             "2026-03-18T15:00:00",
						TotalPage:                2,
					},
					{
						AppointmentID:  "apt-2",
						BusinessID:     "biz-1",
						BusinessName:   "Salon One",
						ServiceTitle:   "Color",
						StartTimeUTC:   "2026-03-19T15:00:00",
						EndTimeUTC:     "2026-03-19T16:30:00",
						AppStatusTitle: "Accepted",
						TotalPage:      2,
					},
				},
			})
		case 2:
			writeJSON(t, w, fetchAppointmentsResponse{
				Status:       http.StatusOK,
				ResponseCode: successResponseCode,
				Message:      "Success",
				Data: []appointmentPayload{
					{
						AppointmentID:            "apt-3",
						BusinessID:               "biz-2",
						BusinessName:             "Spa Two",
						ServiceTitle:             "Massage - 90 min",
						ServiceProviderFirstName: "Jordan",
						ServiceProviderLastName:  "Lee",
						AppStatusTitle:           "Accepted",
						StartTimeUTC:             "2026-03-20T15:00:00",
						TotalPage:                2,
					},
				},
			})
		default:
			t.Fatalf("unexpected page number %d", req.PageNumber)
		}
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: server.Client(),
		sUToken:    "token",
		userAgent:  "test-agent",
	}

	got, err := client.FetchUpcomingAppointments(context.Background(), 2)
	if err != nil {
		t.Fatalf("FetchUpcomingAppointments() error = %v", err)
	}

	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	if got[0].Title != "Haircut - 45 mins @ Salon One" {
		t.Fatalf("got[0].Title = %q", got[0].Title)
	}
	if got[0].Notes != "Business: Salon One\nStaff: Taylor Smith\nStatus: Accepted" {
		t.Fatalf("got[0].Notes = %q", got[0].Notes)
	}
	if got[0].EndTimeUTC.Sub(got[0].StartTimeUTC) != defaultDuration {
		t.Fatalf("got[0] duration = %s, want %s", got[0].EndTimeUTC.Sub(got[0].StartTimeUTC), defaultDuration)
	}
	if got[1].EndTimeUTC.Sub(got[1].StartTimeUTC) != 90*time.Minute {
		t.Fatalf("got[1] duration = %s, want 90m", got[1].EndTimeUTC.Sub(got[1].StartTimeUTC))
	}
}

func TestFetchUpcomingAppointmentsDefaultsDurationWhenEndTimeMissing(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, fetchAppointmentsResponse{
			Status:       http.StatusOK,
			ResponseCode: successResponseCode,
			Message:      "Success",
			Data: []appointmentPayload{
				{
					AppointmentID:  "apt-1",
					BusinessID:     "biz-1",
					BusinessName:   "Salon One",
					ServiceTitle:   "Haircut",
					AppStatusTitle: "Accepted",
					StartTimeUTC:   "2026-03-18T15:00:00",
					TotalPage:      1,
				},
			},
		})
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		httpClient: server.Client(),
		sUToken:    "token",
	}

	got, err := client.FetchUpcomingAppointments(context.Background(), 1)
	if err != nil {
		t.Fatalf("FetchUpcomingAppointments() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].EndTimeUTC.Sub(got[0].StartTimeUTC) != defaultDuration {
		t.Fatalf("duration = %s, want %s", got[0].EndTimeUTC.Sub(got[0].StartTimeUTC), defaultDuration)
	}
}

func TestNormalizeAppointmentsIgnoresVolatileFieldsOutsideTypedPayload(t *testing.T) {
	t.Parallel()

	first := appointmentPayload{
		AppointmentID:            "apt-1",
		BusinessID:               "biz-1",
		BusinessName:             "Salon One",
		ServiceTitle:             "Haircut",
		ServiceProviderFirstName: "Taylor",
		ServiceProviderLastName:  "Smith",
		AppStatusTitle:           "Accepted",
		StartTimeUTC:             "2026-03-18T15:00:00",
		TotalPage:                1,
	}
	second := appointmentPayload{
		AppointmentID:            "apt-1",
		BusinessID:               "biz-1",
		BusinessName:             "Salon One",
		ServiceTitle:             "Haircut",
		ServiceProviderFirstName: "Taylor",
		ServiceProviderLastName:  "Smith",
		AppStatusTitle:           "Accepted",
		StartTimeUTC:             "2026-03-18T15:00:00",
		TotalPage:                99,
	}

	firstAppointments, err := NormalizeAppointments([]appointmentPayload{first})
	if err != nil {
		t.Fatalf("NormalizeAppointments(first) error = %v", err)
	}
	secondAppointments, err := NormalizeAppointments([]appointmentPayload{second})
	if err != nil {
		t.Fatalf("NormalizeAppointments(second) error = %v", err)
	}

	if firstAppointments[0].SourceHash != secondAppointments[0].SourceHash {
		t.Fatalf(
			"SourceHash differs across non-sync payload changes: %q != %q",
			firstAppointments[0].SourceHash,
			secondAppointments[0].SourceHash,
		)
	}
}

func TestNormalizeAppointmentsProviderChangeUpdatesSourceHash(t *testing.T) {
	t.Parallel()

	first := appointmentPayload{
		AppointmentID:            "apt-1",
		BusinessID:               "biz-1",
		BusinessName:             "Salon One",
		ServiceTitle:             "Haircut",
		ServiceProviderFirstName: "Taylor",
		ServiceProviderLastName:  "Smith",
		AppStatusTitle:           "Accepted",
		StartTimeUTC:             "2026-03-18T15:00:00",
		TotalPage:                1,
	}
	second := appointmentPayload{
		AppointmentID:            "apt-1",
		BusinessID:               "biz-1",
		BusinessName:             "Salon One",
		ServiceTitle:             "Haircut",
		ServiceProviderFirstName: "Jordan",
		ServiceProviderLastName:  "Lee",
		AppStatusTitle:           "Accepted",
		StartTimeUTC:             "2026-03-18T15:00:00",
		TotalPage:                1,
	}

	firstAppointments, err := NormalizeAppointments([]appointmentPayload{first})
	if err != nil {
		t.Fatalf("NormalizeAppointments(first) error = %v", err)
	}
	secondAppointments, err := NormalizeAppointments([]appointmentPayload{second})
	if err != nil {
		t.Fatalf("NormalizeAppointments(second) error = %v", err)
	}

	if firstAppointments[0].SourceHash == secondAppointments[0].SourceHash {
		t.Fatalf("SourceHash did not change when provider changed")
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
}
