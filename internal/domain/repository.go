package domain

import "context"

type EventRepository interface {
	Insert(ctx context.Context, event NotificationEvent) error
	FindByStatus(ctx context.Context, status EventStatus) ([]NotificationEvent, error)
	FindByStatuses(ctx context.Context, statuses ...EventStatus) ([]NotificationEvent, error)
	UpdateStatus(ctx context.Context, id string, status EventStatus, retryCount int) error
}
