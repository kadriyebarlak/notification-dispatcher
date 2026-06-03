package service

import (
	"context"
	"errors"
	"testing"

	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type fakeEventRepository struct {
	insertErr       error
	updateStatusErr error
	insertedEvents  []domain.NotificationEvent
	updatedStatuses []domain.EventStatus
}

func (f *fakeEventRepository) Insert(ctx context.Context, event domain.NotificationEvent) error {
	f.insertedEvents = append(f.insertedEvents, event)
	return f.insertErr
}

func (f *fakeEventRepository) UpdateStatus(ctx context.Context, id string, status domain.EventStatus, retryCount int) error {
	f.updatedStatuses = append(f.updatedStatuses, status)
	return f.updateStatusErr
}

// implement remaining interface methods with empty returns
func (f *fakeEventRepository) FindByStatus(ctx context.Context, status domain.EventStatus) ([]domain.NotificationEvent, error) {
	return nil, nil
}

func (f *fakeEventRepository) FindByStatuses(ctx context.Context, statuses ...domain.EventStatus) ([]domain.NotificationEvent, error) {
	return nil, nil
}

func TestEventService_Create(t *testing.T) {
	insertErr := errors.New("insert failed")

	tests := []struct {
		name          string
		eventType     string
		payload       string
		insertErr     error
		wantErr       error
		wantInsertLen int
	}{
		{
			name:          "valid event inserted successfully",
			eventType:     "email",
			payload:       "hello",
			insertErr:     nil,
			wantErr:       nil,
			wantInsertLen: 1,
		},
		{
			name:          "returns error when insert fails",
			eventType:     "email",
			payload:       "hello",
			insertErr:     insertErr,
			wantErr:       insertErr,
			wantInsertLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeEventRepository{
				insertErr: tt.insertErr,
			}

			service := NewEventService(repo)

			err := service.Create(context.Background(), tt.eventType, tt.payload)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("got error %v, want %v", err, tt.wantErr)
			}

			if len(repo.insertedEvents) != tt.wantInsertLen {
				t.Errorf("got inserted events length %d, want %d", len(repo.insertedEvents), tt.wantInsertLen)
			}

			if len(repo.insertedEvents) == 0 {
				return
			}

			insertedEvent := repo.insertedEvents[0]

			if insertedEvent.Type != domain.EventType(tt.eventType) {
				t.Errorf("got event type %q, want %q", insertedEvent.Type, tt.eventType)
			}

			if insertedEvent.Payload != tt.payload {
				t.Errorf("got payload %q, want %q", insertedEvent.Payload, tt.payload)
			}

			if insertedEvent.Status != domain.StatusPending {
				t.Errorf("got status %q, want %q", insertedEvent.Status, domain.StatusPending)
			}

			if insertedEvent.RetryCount != 0 {
				t.Errorf("got retry count %d, want %d", insertedEvent.RetryCount, 0)
			}
		})
	}
}
