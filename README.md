# Notification Dispatcher

A backend service for receiving and dispatching notification events
through multiple channels (email, webhook).

Built in Go as a learning project while transitioning from Java/Spring Boot to Go.

## What it does

Receives notification events via a REST API and dispatches them
asynchronously through multiple channels such as email and webhook.

Events are stored in PostgreSQL with status tracking and automatic
retry on failure. A worker pool processes events concurrently using
Go's native goroutines and channels.

## Project structure

```text
notification-dispatcher/
├── cmd/server/          ← entry point
├── internal/
│   ├── config/          ← environment-based configuration
│   ├── domain/          ← core types and interfaces
│   ├── handler/         ← HTTP handlers and middleware
│   ├── service/         ← business logic
│   ├── repository/      ← PostgreSQL implementation
│   ├── notifier/        ← email and webhook notifiers
│   ├── worker/          ← worker pool
│   └── dispatcher/      ← polling dispatcher
├── migrations/          ← SQL migration files
└── docs/learning/       ← daily learning notes
```

## Architecture
HTTP Request → EventHandler → EventService → EventRepository (PostgreSQL)
↑
Dispatcher (polls every 30s) → WorkerPool → Process → NotifierRegistry
↓
FakeEmailNotifier / FakeWebhookNotifier

## API

| Method | Path | Description |
|--------|------|-------------|
| POST | /events | Submit a notification event |
| GET | /events?status=pending | List events by status |

### Request example

```bash
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{"type":"email","payload":"hello world"}'
```

### Response example

```json
{"status": "accepted"}
```

## How to run

```bash
# start PostgreSQL
docker-compose up -d

# run migrations
make migrate-up

# start the server
make run
```

## Configuration

Configuration is loaded from environment variables with sensible local defaults.

See `.env.example` for all available values:

```env
DATABASE_URL=postgres://notify:notify@localhost:5432/notification_dispatcher?sslmode=disable
PORT=:8080
WORKER_COUNT=3
MAX_RETRIES=3
DISPATCHER_INTERVAL=30s
```

Override any value at runtime:

```bash
PORT=:9090 WORKER_COUNT=5 make run
```

## Event lifecycle
PENDING → PROCESSING → DELIVERED
↘ FAILED (retried up to MAX_RETRIES times)
↘ DEAD (no more retries)

## Tech stack

- Go 1.24
- PostgreSQL 16
- chi (HTTP router)
- pgx v5 (PostgreSQL driver)
- goose (migrations)
- Docker Compose

## Design decisions

- **No ORM** — plain SQL with pgx for full visibility into every query
- **Manual dependency injection** — no framework, all wiring explicit in main.go
- **Interface-based layers** — every layer is testable in isolation with fake implementations
- **Worker pool backed by buffered channel** — HTTP handling and event dispatch are fully decoupled
- **Graceful shutdown** — SIGINT/SIGTERM stops new work and drains in-flight jobs before exit
- **Environment-based config** — no hardcoded values, 12-factor app style

## Known limitations

- Notifiers are fake implementations — no real email or webhook delivery
- Single instance only — multi-instance scaling requires `SELECT FOR UPDATE SKIP LOCKED`
- Worker pool `select` pattern may skip buffered jobs on shutdown — drain-to-empty `range` pattern is the production fix

## Running tests

```bash
# run all tests
go test ./...

# run with race detector
go test -race ./...
```

## Status

- Week 1 ✓ — Go fundamentals, project structure, domain types
- Week 2 ✓ — HTTP API, middleware, PostgreSQL, service layer
- Week 3 ✓ — Worker pool, dispatcher, retry logic, graceful shutdown
- Week 4 🔄 — Tests, production readiness, GitHub polish