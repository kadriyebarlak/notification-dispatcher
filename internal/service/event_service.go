package service

import (
	"context"

	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type EventService struct {
	repo domain.EventRepository
}

func NewEventService(repo domain.EventRepository) *EventService {
	return &EventService{repo: repo}
}

func (s *EventService) Create(ctx context.Context, eventType, payload string) error {
	event := domain.NotificationEvent{
		Type:       domain.EventType(eventType),
		Payload:    payload,
		Status:     domain.StatusPending,
		RetryCount: 0,
	}
	return s.repo.Insert(ctx, event)
}

func (s *EventService) ListByStatus(ctx context.Context, status string) ([]domain.NotificationEvent, error) {
	return s.repo.FindByStatus(ctx, domain.EventStatus(status))
}
