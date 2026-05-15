package main

import (
	"errors"
	"fmt"

	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
	"github.com/kadriyebarlak/notification-dispatcher/internal/notifier"
)

func main() {
	var _ domain.Notifier = notifier.EmailNotifier{}
	var _ domain.Notifier = notifier.WebhookNotifier{}

	fmt.Println("notification dispatcher starting...")

	eventIDs := []string{"", "fail", "evt-001"}

	for _, eventID := range eventIDs {
		err := simulateDispatch(eventID)
		if err == nil {
			fmt.Println("dispatch succeeded for event:", eventID)
			continue
		}

		if errors.Is(err, domain.ErrEventNotFound) {
			fmt.Println("event not found:", err)
			continue
		}

		var notifyErr *domain.NotifyError
		if errors.As(err, &notifyErr) {
			fmt.Println("notify error:")
			fmt.Println("event id:", notifyErr.EventID)
			fmt.Println("reason:", notifyErr.Reason)
			continue
		}

		fmt.Println("unknown error:", err)
	}
}

func simulateDispatch(eventID string) error {
	if eventID == "" {
		return fmt.Errorf("simulateDispatch: %w", domain.ErrEventNotFound)
	}

	if eventID == "fail" {
		return &domain.NotifyError{
			EventID: eventID,
			Reason:  "connection refused",
		}
	}

	return nil
}
