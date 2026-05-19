# Notification Dispatcher

A backend service for receiving and dispatching notification events through multiple channels (email, webhook).

Built in Go as a learning project while transitioning from Java/Spring Boot to Go.

## What it does
Receives notification events via a REST API and dispatches them asynchronously through multiple channels such as email and webhook.

Events are stored in PostgreSQL with status tracking and automatic retry on failure. A worker pool processes events concurrently using Go's native goroutines and channels.

## How to run
make run

## Status
Week 1 complete — project foundation.
Week 2 in progress — HTTP API and persistence.