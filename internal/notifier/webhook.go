package notifier

import (
	"context"
	"log"

	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type FakeWebhookNotifier struct {
	ShouldFail bool
}

func (n *FakeWebhookNotifier) Send(ctx context.Context, event domain.NotificationEvent) error {
	if n.ShouldFail {
		return &domain.NotifyError{
			EventID: event.ID,
			Reason:  "simulated webhook failure",
		}
	}
	log.Printf("webhook sent for event %s type %s", event.ID, event.Type)
	return nil
}
