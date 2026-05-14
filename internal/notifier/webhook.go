package notifier

import (
	"context"
	"fmt"

	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type WebhookNotifier struct{}

func (w WebhookNotifier) Send(ctx context.Context, event domain.NotificationEvent) error {
	fmt.Println("sending webhook notification:", event.ID)
	return nil
}
