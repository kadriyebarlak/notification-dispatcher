package dispatcher

import (
	"context"
	"log"
	"time"

	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
	"github.com/kadriyebarlak/notification-dispatcher/internal/notifier"
	"github.com/kadriyebarlak/notification-dispatcher/internal/worker"
)

type Dispatcher struct {
	repo       domain.EventRepository
	pool       *worker.WorkerPool
	registry   *notifier.NotifierRegistry
	interval   time.Duration
	maxRetries int
}

func NewDispatcher(
	repo domain.EventRepository,
	pool *worker.WorkerPool,
	registry *notifier.NotifierRegistry,
	interval time.Duration,
	maxRetries int,
) *Dispatcher {
	return &Dispatcher{
		repo:       repo,
		pool:       pool,
		registry:   registry,
		interval:   interval,
		maxRetries: maxRetries,
	}
}

func (d *Dispatcher) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(d.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				d.dispatch(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (d *Dispatcher) dispatch(ctx context.Context) {
	events, err := d.repo.FindByStatuses(ctx, domain.StatusPending, domain.StatusFailed)
	if err != nil {
		log.Printf("dispatcher: failed to fetch events: %v", err)
		return
	}

	for _, event := range events {
		if err := d.repo.UpdateStatus(ctx, event.ID, domain.StatusProcessing, event.RetryCount); err != nil {
			log.Printf("dispatcher: failed to update status: %v", err)
			continue
		}
		d.pool.Submit(event)
	}
}

func (d *Dispatcher) Process(ctx context.Context, event domain.NotificationEvent) {
	notifier, ok := d.registry.Get(event.Type)
	if !ok {
		log.Printf("dispatcher: no notifier for type %s", event.Type)
		d.repo.UpdateStatus(ctx, event.ID, domain.StatusDead, event.RetryCount)
		return
	}

	if err := notifier.Send(ctx, event); err != nil {
		log.Printf("dispatcher: failed to send %s (attempt %d): %v", event.ID, event.RetryCount+1, err)

		if event.RetryCount+1 >= d.maxRetries {
			d.repo.UpdateStatus(ctx, event.ID, domain.StatusDead, event.RetryCount+1)
			log.Printf("dispatcher: event %s marked as dead after %d attempts", event.ID, d.maxRetries)
			return
		}

		d.repo.UpdateStatus(ctx, event.ID, domain.StatusFailed, event.RetryCount+1)
		return
	}

	d.repo.UpdateStatus(ctx, event.ID, domain.StatusDelivered, event.RetryCount)
}
