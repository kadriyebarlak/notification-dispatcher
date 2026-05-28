package domain

type NotificationEvent struct {
	ID         string
	Type       EventType
	Payload    string
	Status     EventStatus
	RetryCount int
}

type EventStatus string

const (
	StatusPending    EventStatus = "pending"
	StatusProcessing EventStatus = "processing"
	StatusDelivered  EventStatus = "delivered"
	StatusFailed     EventStatus = "failed"
	StatusDead       EventStatus = "dead"
)

type EventType string

const (
	EventTypeEmail   EventType = "email"
	EventTypeWebhook EventType = "webhook"
)
