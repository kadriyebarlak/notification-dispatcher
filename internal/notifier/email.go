package notifier

import (
	"context"
	"fmt"

	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type EmailNotifier struct{}

func (e EmailNotifier) Send(ctx context.Context, event domain.NotificationEvent) error {
	fmt.Println("sending email notification:", event.ID)
	return nil
}
