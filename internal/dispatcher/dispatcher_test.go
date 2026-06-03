package dispatcher

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
	"github.com/kadriyebarlak/notification-dispatcher/internal/notifier"
)

type updateCall struct {
	id         string
	status     domain.EventStatus
	retryCount int
}

type fakeEventRepository struct {
	updatedStatuses []updateCall
}

func (f *fakeEventRepository) Insert(ctx context.Context, event domain.NotificationEvent) error {
	return nil
}

func (f *fakeEventRepository) UpdateStatus(ctx context.Context, id string, status domain.EventStatus, retryCount int) error {
	f.updatedStatuses = append(f.updatedStatuses, updateCall{
		id:         id,
		status:     status,
		retryCount: retryCount,
	})
	return nil
}

// implement remaining interface methods with empty returns
func (f *fakeEventRepository) FindByStatus(ctx context.Context, status domain.EventStatus) ([]domain.NotificationEvent, error) {
	return nil, nil
}

func (f *fakeEventRepository) FindByStatuses(ctx context.Context, statuses ...domain.EventStatus) ([]domain.NotificationEvent, error) {
	return nil, nil
}

type fakeNotifier struct {
	shouldFail bool
}

func (f *fakeNotifier) Send(ctx context.Context, event domain.NotificationEvent) error {
	if f.shouldFail {
		return errors.New("simulated failure")
	}
	return nil
}

func TestDispatcher_Process(t *testing.T) {
	tests := []struct {
		name           string
		event          domain.NotificationEvent
		registry       *notifier.NotifierRegistry
		maxRetries     int
		wantStatus     domain.EventStatus
		wantRetryCount int
	}{
		{
			name: "no notifier registered marks event as dead",
			event: domain.NotificationEvent{
				ID:         "evt-001",
				Type:       domain.EventTypeEmail,
				Status:     domain.StatusProcessing,
				RetryCount: 0,
			},
			registry:       notifier.NewNotifierRegistry(map[domain.EventType]domain.Notifier{}),
			maxRetries:     3,
			wantStatus:     domain.StatusDead,
			wantRetryCount: 0,
		},
		{
			name: "notifier succeeds marks event as delivered",
			event: domain.NotificationEvent{
				ID:         "evt-002",
				Type:       domain.EventTypeEmail,
				Status:     domain.StatusProcessing,
				RetryCount: 0,
			},
			registry: notifier.NewNotifierRegistry(map[domain.EventType]domain.Notifier{
				domain.EventTypeEmail: &fakeNotifier{shouldFail: false},
			}),
			maxRetries:     3,
			wantStatus:     domain.StatusDelivered,
			wantRetryCount: 0,
		},
		{
			name: "notifier fails below max retries marks event as failed",
			event: domain.NotificationEvent{
				ID:         "evt-003",
				Type:       domain.EventTypeEmail,
				Status:     domain.StatusProcessing,
				RetryCount: 1,
			},
			registry: notifier.NewNotifierRegistry(map[domain.EventType]domain.Notifier{
				domain.EventTypeEmail: &fakeNotifier{shouldFail: true},
			}),
			maxRetries:     3,
			wantStatus:     domain.StatusFailed,
			wantRetryCount: 2,
		},
		{
			name: "notifier fails at max retries marks event as dead",
			event: domain.NotificationEvent{
				ID:         "evt-004",
				Type:       domain.EventTypeEmail,
				Status:     domain.StatusProcessing,
				RetryCount: 2,
			},
			registry: notifier.NewNotifierRegistry(map[domain.EventType]domain.Notifier{
				domain.EventTypeEmail: &fakeNotifier{shouldFail: true},
			}),
			maxRetries:     3,
			wantStatus:     domain.StatusDead,
			wantRetryCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeEventRepository{}

			dispatcher := NewDispatcher(
				repo,
				nil,
				tt.registry,
				time.Second,
				tt.maxRetries,
			)

			dispatcher.Process(context.Background(), tt.event)

			if len(repo.updatedStatuses) != 1 {
				t.Fatalf("got %d update calls, want 1", len(repo.updatedStatuses))
			}

			got := repo.updatedStatuses[0]

			if got.id != tt.event.ID {
				t.Errorf("got id %q, want %q", got.id, tt.event.ID)
			}

			if got.status != tt.wantStatus {
				t.Errorf("got status %q, want %q", got.status, tt.wantStatus)
			}

			if got.retryCount != tt.wantRetryCount {
				t.Errorf("got retry count %d, want %d", got.retryCount, tt.wantRetryCount)
			}
		})
	}
}
