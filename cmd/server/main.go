package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
	"github.com/kadriyebarlak/notification-dispatcher/internal/handler"
	"github.com/kadriyebarlak/notification-dispatcher/internal/notifier"
)

func main() {
	var _ domain.Notifier = notifier.EmailNotifier{}
	var _ domain.Notifier = notifier.WebhookNotifier{}

	fmt.Println("notification dispatcher starting...")

	eventHandler := handler.EventHandler{}
	r := chi.NewRouter()
	r.Post("/events", eventHandler.CreateEvent)

	fmt.Println("server listening on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}

}
