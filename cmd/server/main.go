package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
	"github.com/kadriyebarlak/notification-dispatcher/internal/handler"
	"github.com/kadriyebarlak/notification-dispatcher/internal/notifier"
	"github.com/kadriyebarlak/notification-dispatcher/internal/repository"
	"github.com/kadriyebarlak/notification-dispatcher/internal/service"
)

func main() {
	var _ domain.Notifier = notifier.EmailNotifier{}
	var _ domain.Notifier = notifier.WebhookNotifier{}

	fmt.Println("notification dispatcher starting...")

	ctx := context.Background()

	dbURL := "postgres://notify:notify@localhost:5432/notification_dispatcher?sslmode=disable"

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal("cannot connect to database:", err)
	}

	eventRepository := repository.NewPostgresEventRepository(pool)
	eventService := service.NewEventService(eventRepository)
	eventHandler := handler.NewEventHandler(eventService)

	r := chi.NewRouter()
	r.Use(handler.LoggingMiddleware)
	r.Use(handler.TimeoutMiddleware(5 * time.Second))
	r.Get("/events", eventHandler.ListEvents)
	r.Post("/events", eventHandler.CreateEvent)

	fmt.Println("server listening on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatal(err)
	}

}
