package domain

import "context"

type Notifier interface {
	Send(ctx context.Context, event NotificationEvent) error
}
