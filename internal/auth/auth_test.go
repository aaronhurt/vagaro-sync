package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aaronhurt/vagaro-sync/internal/storage"
)

func TestProbeAppointmentsSendsCurrentAPIRequest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("Method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q, want application/json", got)
		}
		if got := r.Header.Get("s_utkn"); got != "token" {
			t.Fatalf("s_utkn = %q, want token", got)
		}
		if got := r.Header.Get("User-Agent"); got != "test-agent" {
			t.Fatalf("User-Agent = %q, want test-agent", got)
		}

		var payload appointmentsProbeRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if payload.PageSize != 1 || payload.PageNumber != 1 {
			t.Fatalf("payload pagination = %+v", payload)
		}
		if payload.Device != requestDevice || payload.Module != requestModule || payload.Version != requestVersion {
			t.Fatalf("payload metadata = %+v", payload)
		}
		if payload.PastAppointment || payload.MyOrSharedAppointments != 1 || payload.BrandedApp {
			t.Fatalf("payload flags = %+v", payload)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(appointmentsProbeResponse{
			Status:       http.StatusOK,
			ResponseCode: 1000,
		}); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}))
	defer server.Close()

	bundle := storage.AuthBundle{
		SUToken:   "token",
		UserAgent: "test-agent",
	}

	req, err := newAppointmentsProbeRequest(context.Background(), server.URL, bundle)
	if err != nil {
		t.Fatalf("newAppointmentsProbeRequest() error = %v", err)
	}

	client, err := clientFromBundle(server.URL, bundle)
	if err != nil {
		t.Fatalf("clientFromBundle() error = %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
	}
}
