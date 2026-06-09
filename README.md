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

```
HTTP Request → EventHandler → EventService → EventRepository (PostgreSQL)
                                                      ↑
Dispatcher (polls every 30s) → WorkerPool → Process → NotifierRegistry
                                                      ↓
                                          FakeEmailNotifier / FakeWebhookNotifier
```

## API

### POST /events

Submit a notification event for async dispatch.

**Request:**
```json
{
  "type": "email",
  "payload": "your notification content"
}
```

**Response — 202 Accepted:**
```json
{"status": "accepted"}
```

**Response — 400 Bad Request:**
```json
{"errors": ["type is required", "payload is required"]}
```

**Supported event types:** `email`, `webhook`

---

### GET /events

List events filtered by status.

**Query params:** `status` — one of `pending`, `processing`, `delivered`, `failed`, `dead` (default: `pending`)

**Example:**
```bash
curl http://localhost:8080/events?status=delivered
```

**Response — 200 OK:**
```json
[
  {
    "ID": "evt-1234567890",
    "Type": "email",
    "Payload": "your content",
    "Status": "delivered",
    "RetryCount": 0
  }
]
```

---

### GET /health

Liveness probe. Returns 200 if the process is running.

```json
{"status": "ok"}
```

---

### GET /ready

Readiness probe. Returns 200 if the service and database are healthy, 503 otherwise.

```json
{"status": "ok"}
{"status": "unavailable", "reason": "database unreachable"}
```

---

## How to run

```bash
# start PostgreSQL
docker-compose up -d

# run migrations
make migrate-up

# start the server
make run
```

## How to develop

```bash
# run all tests
make test

# run with race detector
make test-race

# lint
make lint

# format
make fmt

# build Docker image
make docker-build
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

```
PENDING → PROCESSING → DELIVERED
                     ↘ FAILED (retried up to MAX_RETRIES times)
                              ↘ DEAD (no more retries)
```

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

## Java to Go — what I learned building this

I built this service while transitioning from Java/Spring Boot to Go.
The same service in Spring Boot would use:

| Spring Boot | Go |
|---|---|
| `@RestController` + `@Autowired` | plain `http.Handler` + manual constructor injection |
| `ThreadPoolExecutor` + `@Async` | goroutines + buffered channels |
| Spring Batch scheduled jobs | `time.NewTicker` + context cancellation |
| Flyway migrations | goose |
| Fat JAR (~250MB Docker image) | static binary (34MB Docker image) |

The most important mindset shift: Go has no magic.
Every dependency is wired explicitly. Every error is handled explicitly.
Every goroutine is started explicitly. This makes the code more verbose
but also more transparent — you can always see exactly what is happening and why.

## Known limitations

- Notifiers are fake implementations — no real email or webhook delivery
- Single instance only — multi-instance scaling requires `SELECT FOR UPDATE SKIP LOCKED`
- Worker pool `select` pattern may skip buffered jobs on shutdown — drain-to-empty `range` pattern is the production fix