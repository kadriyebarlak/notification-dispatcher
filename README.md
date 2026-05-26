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
│   ├── domain/          ← core types and interfaces
│   ├── handler/         ← HTTP handlers and middleware
│   ├── service/         ← business logic
│   ├── repository/      ← PostgreSQL implementation
│   ├── notifier/        ← email and webhook notifiers
│   └── worker/          ← worker pool
├── migrations/          ← SQL migration files
└── docs/learning/       ← daily learning notes
```

## API

| Method | Path | Description |
|--------|------|-------------|
| POST | /events | Submit a notification event |
| GET | /events?status=pending | List events by status |

## How to run

```bash
# start PostgreSQL
docker-compose up -d

# run migrations
make migrate-up

# start the server
make run
```

## Tech stack

- Go 1.24
- PostgreSQL 16
- chi (HTTP router)
- pgx (PostgreSQL driver)
- goose (migrations)
- Docker Compose

## Status

- Week 1 ✓ — Go fundamentals, project structure, domain types
- Week 2 ✓ — HTTP API, middleware, PostgreSQL, service layer
- Week 3 🔄 — Worker pool, notifier dispatch, retry, graceful shutdown
- Week 4 — Tests, production readiness, GitHub polish