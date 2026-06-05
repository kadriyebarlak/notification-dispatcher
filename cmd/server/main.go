package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kadriyebarlak/notification-dispatcher/internal/config"
	"github.com/kadriyebarlak/notification-dispatcher/internal/dispatcher"
	"github.com/kadriyebarlak/notification-dispatcher/internal/domain"
	"github.com/kadriyebarlak/notification-dispatcher/internal/handler"
	"github.com/kadriyebarlak/notification-dispatcher/internal/notifier"
	"github.com/kadriyebarlak/notification-dispatcher/internal/repository"
	"github.com/kadriyebarlak/notification-dispatcher/internal/service"
	"github.com/kadriyebarlak/notification-dispatcher/internal/worker"
)

func main() {
	var _ domain.Notifier = (*notifier.FakeEmailNotifier)(nil)
	var _ domain.Notifier = (*notifier.FakeWebhookNotifier)(nil)

	cfg := config.LoadConfig()

	fmt.Println("notification dispatcher starting...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}

	if err := pool.Ping(ctx); err != nil {
		log.Fatal("cannot connect to database:", err)
	}

	eventRepository := repository.NewPostgresEventRepository(pool)
	eventService := service.NewEventService(eventRepository)
	eventHandler := handler.NewEventHandler(eventService)

	registry := notifier.NewNotifierRegistry(map[domain.EventType]domain.Notifier{
		domain.EventTypeEmail:   &notifier.FakeEmailNotifier{},
		domain.EventTypeWebhook: &notifier.FakeWebhookNotifier{},
	})
	workerPool := worker.NewWorkerPool(cfg.WorkerCount)
	disp := dispatcher.NewDispatcher(eventRepository, workerPool, registry, cfg.DispatcherInterval, cfg.MaxRetries)

	workerPool.Start(ctx, disp.Process)
	disp.Start(ctx)

	r := chi.NewRouter()
	r.Use(handler.LoggingMiddleware)
	r.Use(handler.TimeoutMiddleware(5 * time.Second))
	r.Get("/events", eventHandler.ListEvents)
	r.Post("/events", eventHandler.CreateEvent)
	r.Get("/health", handler.HealthHandler)
	r.Get("/ready", handler.NewReadinessHandler(pool).Ready)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}
	// start in a goroutine
	go func() {
		log.Println("server listening on " + cfg.Port)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error:", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// wait for signal
	<-quit
	log.Println("shutdown signal received")

	// 1. cancel context — stops dispatcher and workers from accepting new work
	cancel()

	// 2. stop HTTP server — no new requests
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}

	// 3. stop worker pool — wait for in-flight jobs
	workerPool.Stop()

	// 4. close DB pool — clean disconnect
	pool.Close()

	log.Println("shutdown complete")

}
