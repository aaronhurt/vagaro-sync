// Package auth handles browser-assisted authentication and session validation.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aaronhurt/vagaro-sync/internal/browser"
	"github.com/aaronhurt/vagaro-sync/internal/storage"
)

const (
	loginURL             = "https://www.vagaro.com/login"
	appointmentsProbeURL = "https://api.vagaro.com/us02/api/v2/myaccount/purchases/appointments"
	requestDevice        = "Website"
	requestModule        = "MyAccount"
	requestVersion       = "2.5.3"
)

type appointmentsProbeRequest struct {
	PageSize               int    `json:"pageSize"`
	PageNumber             int    `json:"pageNumber"`
	PastAppointment        bool   `json:"pastAppointment"`
	MyOrSharedAppointments int    `json:"myOrSharedAppointments"`
	BrandedApp             bool   `json:"brandedApp"`
	MerchantID             string `json:"merchantId"`
	AppNo                  *int   `json:"appNo"`
	Device                 string `json:"device"`
	Module                 string `json:"module"`
	Version                string `json:"version"`
}

type appointmentsProbeResponse struct {
	Status       int `json:"status"`
	ResponseCode int `json:"responseCode"`
}

// Login launches provider-agnostic browser authentication and returns the captured session bundle.
func Login(ctx context.Context, backend browser.Backend) (storage.AuthBundle, error) {
	return backend.Authenticate(ctx, loginURL)
}

// ProbeAppointments validates a captured session by probing the Vagaro appointments endpoint.
func ProbeAppointments(ctx context.Context, bundle storage.AuthBundle) error {
	req, err := newAppointmentsProbeRequest(ctx, appointmentsProbeURL, bundle)
	if err != nil {
		return err
	}

	client, err := clientFromBundle(appointmentsProbeURL, bundle)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("run appointments probe: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read appointments probe response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("appointments probe returned %s", resp.Status)
	}

	var probe appointmentsProbeResponse
	if err := json.Unmarshal(body, &probe); err != nil {
		return fmt.Errorf("decode appointments probe response: %w", err)
	}
	if probe.Status != http.StatusOK || probe.ResponseCode != 1000 {
		return fmt.Errorf(
			"appointments probe returned status=%d responseCode=%d",
			probe.Status,
			probe.ResponseCode,
		)
	}

	return nil
}

func newAppointmentsProbeRequest(ctx context.Context, endpoint string, bundle storage.AuthBundle) (*http.Request, error) {
	payload, err := json.Marshal(appointmentsProbeRequest{
		PageSize:               1,
		PageNumber:             1,
		PastAppointment:        false,
		MyOrSharedAppointments: 1,
		BrandedApp:             false,
		MerchantID:             "",
		AppNo:                  nil,
		Device:                 requestDevice,
		Module:                 requestModule,
		Version:                requestVersion,
	})
	if err != nil {
		return nil, fmt.Errorf("encode appointments probe request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build appointments probe request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Origin", "https://www.vagaro.com")
	req.Header.Set("Referer", "https://www.vagaro.com/")
	req.Header.Set("s_utkn", bundle.SUToken)
	if bundle.UserAgent != "" {
		req.Header.Set("User-Agent", bundle.UserAgent)
	}

	return req, nil
}

func clientFromBundle(_ string, _ storage.AuthBundle) (*http.Client, error) {
	return &http.Client{
		Timeout: 30 * time.Second,
	}, nil
}
