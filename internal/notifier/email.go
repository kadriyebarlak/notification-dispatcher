package notifier

import (
	"context"
	"log"

	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type FakeEmailNotifier struct {
	ShouldFail bool
}

func (n *FakeEmailNotifier) Send(ctx context.Context, event domain.NotificationEvent) error {
	if n.ShouldFail {
		return &domain.NotifyError{
			EventID: event.ID,
			Reason:  "simulated email failure",
		}
	}
	log.Printf("email sent for event %s type %s", event.ID, event.Type)
	return nil
}
