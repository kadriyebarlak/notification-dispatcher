package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type fakeEventService struct {
	createErr    error
	listEvents   []domain.NotificationEvent
	listErr      error
	calledStatus string
}

func (f *fakeEventService) Create(ctx context.Context, eventType, payload string) error {
	return f.createErr
}

func (f *fakeEventService) ListByStatus(ctx context.Context, status string) ([]domain.NotificationEvent, error) {
	f.calledStatus = status
	return f.listEvents, f.listErr
}

func TestCreateEvent(t *testing.T) {
	tests := []struct {
		name            string
		body            string
		mockErr         error
		wantStatus      int
		wantBody        map[string]string
		wantContentType string
	}{
		{
			name:            "valid request",
			body:            `{"type":"email","payload":"hello"}`,
			mockErr:         nil,
			wantStatus:      http.StatusAccepted,
			wantBody:        map[string]string{"status": "accepted"},
			wantContentType: "application/json",
		},
		{
			name:       "missing type",
			body:       `{"payload":"hello"}`,
			mockErr:    nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing payload",
			body:       `{"type":"email"}`,
			mockErr:    nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing type and payload",
			body:       `{}`,
			mockErr:    nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid json",
			body:       `{"type":`,
			mockErr:    nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:            "service error",
			body:            `{"type":"email","payload":"hello"}`,
			mockErr:         errors.New("database down"),
			wantStatus:      http.StatusInternalServerError,
			wantBody:        map[string]string{"error": "failed to create event"},
			wantContentType: "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. fake service
			service := fakeEventService{
				createErr: tt.mockErr,
			}
			// 2. create handler with fake
			handler := NewEventHandler(&service)
			// 3. create test request and recorder
			req := httptest.NewRequest(
				http.MethodPost,
				"/events",
				strings.NewReader(tt.body),
			)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			// 4. call handler
			handler.CreateEvent(rec, req)
			// 5. check status code
			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantContentType != "" {
				if ct := rec.Header().Get("Content-Type"); ct != tt.wantContentType {
					t.Errorf("got Content-Type %q, want %q", ct, tt.wantContentType)
				}
			}

			if tt.wantBody != nil {
				var gotBody map[string]string

				if err := json.NewDecoder(rec.Body).Decode(&gotBody); err != nil {
					t.Fatalf("could not decode response body: %v", err)
				}

				for key, wantValue := range tt.wantBody {
					if gotBody[key] != wantValue {
						t.Errorf("got body[%q] %q, want %q", key, gotBody[key], wantValue)
					}
				}
			}

		})
	}
}

func TestListEvents(t *testing.T) {
	tests := []struct {
		name             string
		url              string
		listEvents       []domain.NotificationEvent
		listErr          error
		wantStatus       int
		wantCalledStatus string
		wantError        string
		wantEventCount   int
	}{
		{
			name: "returns events for given status",
			url:  "/events?status=failed",
			listEvents: []domain.NotificationEvent{
				{
					ID:         "evt-001",
					Type:       domain.EventTypeEmail,
					Payload:    "hello",
					Status:     domain.StatusFailed,
					RetryCount: 1,
				},
				{
					ID:         "evt-002",
					Type:       domain.EventTypeWebhook,
					Payload:    "webhook payload",
					Status:     domain.StatusFailed,
					RetryCount: 2,
				},
			},
			listErr:          nil,
			wantStatus:       http.StatusOK,
			wantCalledStatus: string(domain.StatusFailed),
			wantEventCount:   2,
		},
		{
			name: "defaults to pending when no status given",
			url:  "/events",
			listEvents: []domain.NotificationEvent{
				{
					ID:         "evt-003",
					Type:       domain.EventTypeEmail,
					Payload:    "hello",
					Status:     domain.StatusPending,
					RetryCount: 0,
				},
			},
			listErr:          nil,
			wantStatus:       http.StatusOK,
			wantCalledStatus: string(domain.StatusPending),
			wantEventCount:   1,
		},
		{
			name:             "service error returns 500",
			url:              "/events?status=pending",
			listEvents:       nil,
			listErr:          errors.New("database down"),
			wantStatus:       http.StatusInternalServerError,
			wantCalledStatus: string(domain.StatusPending),
			wantError:        "failed to list events",
			wantEventCount:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeEventService{
				listEvents: tt.listEvents,
				listErr:    tt.listErr,
			}

			handler := NewEventHandler(service)

			req := httptest.NewRequest(
				http.MethodGet,
				tt.url,
				nil,
			)

			rec := httptest.NewRecorder()

			handler.ListEvents(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}

			if service.calledStatus != tt.wantCalledStatus {
				t.Errorf("got called status %q, want %q", service.calledStatus, tt.wantCalledStatus)
			}

			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("got Content-Type %q, want %q", ct, "application/json")
			}

			if tt.wantError != "" {
				var body map[string]string

				if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
					t.Fatalf("could not decode response body: %v", err)
				}

				if body["error"] != tt.wantError {
					t.Errorf("got error %q, want %q", body["error"], tt.wantError)
				}

				return
			}

			var events []domain.NotificationEvent

			if err := json.NewDecoder(rec.Body).Decode(&events); err != nil {
				t.Fatalf("could not decode response body: %v", err)
			}

			if len(events) != tt.wantEventCount {
				t.Errorf("got event count %d, want %d", len(events), tt.wantEventCount)
			}
		})
	}
}
