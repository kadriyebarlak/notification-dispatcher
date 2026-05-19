package main

import (
	"fmt"

	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
	"github.com/kadriyebarlak/notification-dispatcher/internal/notifier"
)

func main() {
	var _ domain.Notifier = notifier.EmailNotifier{}
	var _ domain.Notifier = notifier.WebhookNotifier{}

	fmt.Println("notification dispatcher starting...")

}
