package domain

import (
	"errors"
	"fmt"
)

var ErrEventNotFound = errors.New("event not found")

type NotifyError struct {
	EventID string
	Reason  string
}

func (e *NotifyError) Error() string {
	return fmt.Sprintf("notification failed for event %s: %s", e.EventID, e.Reason)
}
