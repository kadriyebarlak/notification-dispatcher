package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type fakeEventService struct {
	err error
}

func (f fakeEventService) Create(ctx context.Context, eventType, payload string) error {
	return f.err
}

func (f fakeEventService) ListByStatus(ctx context.Context, status string) ([]domain.NotificationEvent, error) {
	return nil, nil
}

func TestCreateEvent(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		mockErr    error
		wantStatus int
	}{
		{
			name:       "valid request",
			body:       `{"type":"email","payload":"hello"}`,
			mockErr:    nil,
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "missing type",
			body:       `{"payload":"hello"}`,
			mockErr:    nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "service error",
			body:       `{"type":"email","payload":"hello"}`,
			mockErr:    errors.New("database down"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. fake service
			service := fakeEventService{
				err: tt.mockErr,
			}
			// 2. create handler with fake
			handler := NewEventHandler(service)
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
		})
	}
}
