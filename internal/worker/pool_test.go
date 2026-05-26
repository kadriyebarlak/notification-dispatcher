package worker

import (
	"context"
	"sync"
	"testing"

	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

func TestWorkerPoolProcessesEvents(t *testing.T) {
	tests := []struct {
		name        string
		workerCount int
		events      []domain.NotificationEvent
		wantCount   int
	}{
		{
			name:        "processes all submitted events",
			workerCount: 3,
			events: []domain.NotificationEvent{
				{ID: "evt-001"},
				{ID: "evt-002"},
				{ID: "evt-003"},
				{ID: "evt-004"},
				{ID: "evt-005"},
			},
			wantCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewWorkerPool(tt.workerCount)

			ctx := context.Background()

			var mu sync.Mutex
			processedCount := 0

			pool.Start(ctx, func(ctx context.Context, event domain.NotificationEvent) {
				mu.Lock()
				processedCount++
				mu.Unlock()
			})

			for _, event := range tt.events {
				pool.Submit(event)
			}

			pool.Stop()

			if processedCount != tt.wantCount {
				t.Errorf("got processed count %d, want %d", processedCount, tt.wantCount)
			}
		})
	}
}
