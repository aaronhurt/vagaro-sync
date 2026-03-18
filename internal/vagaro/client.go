// Package vagaro fetches and normalizes appointment data from Vagaro endpoints.
package vagaro

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aaronhurt/vagaro-sync/internal/storage"
)

const (
	defaultBaseURL           = "https://api.vagaro.com/us02/api/v2"
	defaultPageSize          = 24
	defaultDuration          = time.Hour
	appointmentsEndpointPath = "/myaccount/purchases/appointments"
	requestDevice            = "Website"
	requestModule            = "MyAccount"
	requestVersion           = "2.5.3"
	successResponseCode      = 1000
)

// Client fetches upcoming appointments from Vagaro using a captured auth bundle.
type Client struct {
	baseURL    string
	httpClient *http.Client
	sUToken    string
	userAgent  string
}

type appointmentPayload struct {
	AppointmentID            string `json:"appointmentId"`
	BusinessID               string `json:"businessId"`
	BusinessName             string `json:"businessName"`
	ServiceTitle             string `json:"serviceTitle"`
	ServiceProviderFirstName string `json:"serviceProviderFirstName"`
	ServiceProviderLastName  string `json:"serviceProviderLastName"`
	AppStatusTitle           string `json:"appStatusTitle"`
	StartTime                string `json:"startTime"`
	StartTimeUTC             string `json:"startTimeUTC"`
	EndTime                  string `json:"endTime"`
	EndTimeUTC               string `json:"endTimeUTC"`
	EventType                int    `json:"eventType"`
	Group                    string `json:"sGroup"`
	TotalPage                int    `json:"totalPage"`
}

// Appointment is the normalized appointment model used by sync planning.
type Appointment struct {
	AppointmentID string
	SourceHash    string
	Title         string
	Location      string
	Notes         string
	StartTimeUTC  time.Time
	EndTimeUTC    time.Time
}

type fetchAppointmentsRequest struct {
	PageSize               int    `json:"pageSize"`
	PageNumber             int    `json:"pageNumber"`
	PastAppointment        bool   `json:"pastAppointment"`
	MyOrSharedAppointments int    `json:"myOrSharedAppointments"`
	Device                 string `json:"device"`
	Module                 string `json:"module"`
	Version                string `json:"version"`
	BrandedApp             bool   `json:"brandedApp"`
	MerchantID             string `json:"merchantId"`
	AppNo                  *int   `json:"appNo"`
}

type fetchAppointmentsResponse struct {
	Status       int                  `json:"status"`
	ResponseCode int                  `json:"responseCode"`
	Message      string               `json:"message"`
	Data         []appointmentPayload `json:"data"`
}

type syncHashInput struct {
	AppointmentID            string    `json:"appointment_id"`
	BusinessID               string    `json:"business_id"`
	BusinessName             string    `json:"business_name"`
	ServiceTitle             string    `json:"service_title"`
	ServiceProviderFirstName string    `json:"service_provider_first_name"`
	ServiceProviderLastName  string    `json:"service_provider_last_name"`
	AppStatusTitle           string    `json:"app_status_title"`
	StartTimeRaw             string    `json:"start_time"`
	StartTimeUTCRaw          string    `json:"start_time_utc_raw"`
	EndTimeRaw               string    `json:"end_time"`
	EndTimeUTCRaw            string    `json:"end_time_utc_raw"`
	EventType                int       `json:"event_type"`
	Group                    string    `json:"group"`
	StartTimeUTC             time.Time `json:"start_time_utc"`
	EndTimeUTC               time.Time `json:"end_time_utc"`
}

// NewClient returns a Vagaro appointments client backed by the provided auth bundle.
func NewClient(bundle storage.AuthBundle) (*Client, error) {
	httpClient, err := newHTTPClient()
	if err != nil {
		return nil, err
	}

	return &Client{
		baseURL:    defaultBaseURL,
		httpClient: httpClient,
		sUToken:    bundle.SUToken,
		userAgent:  bundle.UserAgent,
	}, nil
}

// FetchUpcomingAppointments fetches and normalizes upcoming appointments across all pages.
func (c *Client) FetchUpcomingAppointments(ctx context.Context, pageSize int) ([]Appointment, error) {
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}

	var (
		pageNumber   = 1
		appointments []Appointment
	)
	for {
		page, totalPages, err := c.fetchUpcomingAppointmentsPage(ctx, pageNumber, pageSize)
		if err != nil {
			return nil, err
		}

		if len(page) == 0 {
			break
		}

		normalized, err := NormalizeAppointments(page)
		if err != nil {
			return nil, fmt.Errorf("normalize page %d: %w", pageNumber, err)
		}
		appointments = append(appointments, normalized...)

		if totalPages > 0 && pageNumber >= totalPages {
			break
		}
		if len(page) < pageSize {
			break
		}
		pageNumber++
	}

	return appointments, nil
}

func (c *Client) fetchUpcomingAppointmentsPage(
	ctx context.Context,
	pageNumber int,
	pageSize int,
) ([]appointmentPayload, int, error) {
	payload, err := json.Marshal(fetchAppointmentsRequest{
		PageSize:               pageSize,
		PageNumber:             pageNumber,
		PastAppointment:        false,
		MyOrSharedAppointments: 1,
		Device:                 requestDevice,
		Module:                 requestModule,
		Version:                requestVersion,
		BrandedApp:             false,
		MerchantID:             "",
		AppNo:                  nil,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("encode appointments page request: %w", err)
	}

	endpoint := c.baseURL + appointmentsEndpointPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, fmt.Errorf("build appointments page request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Origin", "https://www.vagaro.com")
	req.Header.Set("Referer", "https://www.vagaro.com/")
	req.Header.Set("s_utkn", c.sUToken)
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch appointments page %d: %w", pageNumber, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("read appointments page %d: %w", pageNumber, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("fetch appointments page %d: %s", pageNumber, resp.Status)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, 0, fmt.Errorf("decode appointments page %d: empty response body", pageNumber)
	}

	var decoded fetchAppointmentsResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, 0, fmt.Errorf("decode appointments page %d: %w", pageNumber, err)
	}
	if decoded.Status != http.StatusOK || decoded.ResponseCode != successResponseCode {
		return nil, 0, fmt.Errorf(
			"fetch appointments page %d: status=%d responseCode=%d message=%q",
			pageNumber,
			decoded.Status,
			decoded.ResponseCode,
			decoded.Message,
		)
	}

	return decoded.Data, totalPages(decoded.Data), nil
}

// NormalizeAppointments converts typed Vagaro payloads into the normalized appointment model.
func NormalizeAppointments(payloads []appointmentPayload) ([]Appointment, error) {
	normalized := make([]Appointment, 0, len(payloads))
	for idx, payload := range payloads {
		appointment, err := normalizeAppointment(payload)
		if err != nil {
			return nil, fmt.Errorf("appointment %d: %w", idx, err)
		}
		normalized = append(normalized, appointment)
	}

	return normalized, nil
}

func normalizeAppointment(payload appointmentPayload) (Appointment, error) {
	startTime, err := parseAppointmentTime(firstNonEmpty(payload.StartTimeUTC, payload.StartTime))
	if err != nil {
		return Appointment{}, err
	}
	if startTime.IsZero() {
		return Appointment{}, fmt.Errorf("missing appointment start time")
	}

	endTime, err := parseAppointmentTime(firstNonEmpty(payload.EndTimeUTC, payload.EndTime))
	if err != nil {
		return Appointment{}, err
	}
	if endTime.IsZero() {
		endTime = startTime.Add(defaultDuration)
	}

	staffName := joinNonEmpty(" ", payload.ServiceProviderFirstName, payload.ServiceProviderLastName)
	title := buildTitle(payload.ServiceTitle, payload.BusinessName)
	notes := joinNonEmpty("\n",
		prefixedValue("Business", payload.BusinessName),
		prefixedValue("Staff", staffName),
		prefixedValue("Status", payload.AppStatusTitle),
	)

	appointmentID := strings.TrimSpace(payload.AppointmentID)
	if appointmentID == "" {
		return Appointment{}, fmt.Errorf("missing appointment ID")
	}

	sourceHash, err := hashAppointmentSyncFields(syncHashInput{
		AppointmentID:            payload.AppointmentID,
		BusinessID:               payload.BusinessID,
		BusinessName:             payload.BusinessName,
		ServiceTitle:             payload.ServiceTitle,
		ServiceProviderFirstName: payload.ServiceProviderFirstName,
		ServiceProviderLastName:  payload.ServiceProviderLastName,
		AppStatusTitle:           payload.AppStatusTitle,
		StartTimeRaw:             payload.StartTime,
		StartTimeUTCRaw:          payload.StartTimeUTC,
		EndTimeRaw:               payload.EndTime,
		EndTimeUTCRaw:            payload.EndTimeUTC,
		EventType:                payload.EventType,
		Group:                    payload.Group,
		StartTimeUTC:             startTime,
		EndTimeUTC:               endTime,
	})
	if err != nil {
		return Appointment{}, err
	}

	return Appointment{
		AppointmentID: appointmentID,
		SourceHash:    sourceHash,
		Title:         title,
		Notes:         notes,
		StartTimeUTC:  startTime,
		EndTimeUTC:    endTime,
	}, nil
}

func newHTTPClient() (*http.Client, error) {
	return &http.Client{
		Timeout: 30 * time.Second,
	}, nil
}

func hashAppointmentSyncFields(input syncHashInput) (string, error) {
	data, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("encode sync hash input: %w", err)
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func parseAppointmentTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}

		parsed, err = time.ParseInLocation(layout, value, time.UTC)
		if err == nil {
			return parsed.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("parse time value %q", value)
}

func buildTitle(serviceName string, businessName string) string {
	switch {
	case strings.TrimSpace(serviceName) != "" && strings.TrimSpace(businessName) != "":
		return strings.TrimSpace(serviceName) + " @ " + strings.TrimSpace(businessName)
	case strings.TrimSpace(serviceName) != "":
		return strings.TrimSpace(serviceName)
	case strings.TrimSpace(businessName) != "":
		return strings.TrimSpace(businessName)
	default:
		return "Vagaro Appointment"
	}
}

func joinNonEmpty(separator string, values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}

	return strings.Join(parts, separator)
}

func prefixedValue(prefix string, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}

	return prefix + ": " + strings.TrimSpace(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func totalPages(payloads []appointmentPayload) int {
	if len(payloads) == 0 {
		return 0
	}

	return payloads[0].TotalPage
}
