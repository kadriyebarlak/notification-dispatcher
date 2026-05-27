package notifier

import "github.com/kadriyebarlak/notification-dispatcher/internal/domain"

type NotifierRegistry struct {
	notifiers map[domain.EventType]domain.Notifier
}

func NewNotifierRegistry(notifiers map[domain.EventType]domain.Notifier) *NotifierRegistry {
	return &NotifierRegistry{notifiers: notifiers}
}

func (r *NotifierRegistry) Get(eventType domain.EventType) (domain.Notifier, bool) {
	n, ok := r.notifiers[eventType]
	return n, ok
}
