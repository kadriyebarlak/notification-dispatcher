# Day 14 — Week 2 Review: GET /events, Handler Tests & Docker Compose

---

## 1. Original Lesson Explanation

### 1.1 What Day 14 covers

Day 14 is a review and consolidation day — no big new concepts.
Three things to complete before Week 3:

1. Add `GET /events` endpoint
2. Write handler tests with a mocked service using `httptest`
3. Add Docker Compose for one-command project startup

---

### 1.2 Adding `GET /events`

The API can create events but cannot list them.
`GET /events?status=pending` returns events filtered by status.

**Service interface in handler package — add `ListByStatus`:**
```go
type EventService interface {
    Create(ctx context.Context, eventType, payload string) error
    ListByStatus(ctx context.Context, status string) ([]domain.NotificationEvent, error)
}
```

**Service implementation:**
```go
func (s *EventService) ListByStatus(ctx context.Context, status string) ([]domain.NotificationEvent, error) {
    return s.repo.FindByStatus(ctx, domain.EventStatus(status))
}
```

**Handler:**
```go
func (h *EventHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
    status := r.URL.Query().Get("status")
    if status == "" {
        status = "pending"
    }
    events, err := h.service.ListByStatus(r.Context(), status)
    if err != nil {
        writeError(w, http.StatusInternalServerError, "failed to list events")
        return
    }
    writeJSON(w, http.StatusOK, events)
}
```

**Route:**
```go
r.Get("/events", eventHandler.ListEvents)
```

---

### 1.3 Handler tests with `httptest`

Go's `net/http/httptest` package lets you test HTTP handlers without a running server.

**Two key types:**
- `httptest.NewRequest(method, url, body)` — creates a fake `*http.Request`
- `httptest.NewRecorder()` — creates a fake `http.ResponseWriter` that records everything in memory

**Why `*httptest.ResponseRecorder` can be passed as `http.ResponseWriter`:**

`http.ResponseWriter` is an interface:
```go
type ResponseWriter interface {
    Header() http.Header
    Write([]byte) (int, error)
    WriteHeader(statusCode int)
}
```

`*httptest.ResponseRecorder` implements all three methods — implicit satisfaction, same as Day 2.
The handler does not know or care whether it is talking to a real server writer or a test recorder.
It just calls the interface methods — the recorder captures everything into memory.

**The fake service pattern:**

```go
type fakeEventService struct {
    err error
}

func (f fakeEventService) Create(ctx context.Context, eventType, payload string) error {
    return f.err
}

func (f fakeEventService) ListByStatus(ctx context.Context, status string) ([]domain.NotificationEvent, error) {
    return nil, nil
}
```

The `err` field is configurable per test case — control exactly what the fake returns.

---

### 1.4 Docker Compose

Docker Compose starts all services with one command.
No more manual `docker run` commands.

```yaml
version: "3.9"

services:
  db:
    image: postgres:16
    environment:
      POSTGRES_USER: notify
      POSTGRES_PASSWORD: notify
      POSTGRES_DB: notification_dispatcher
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U notify"]
      interval: 5s
      timeout: 5s
      retries: 5
```

```makefile
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down
```

---

## 2. My Learning Summary

**What I learned:**
- `GET /events` with query param defaulting — `r.URL.Query().Get("status")`
- `httptest.NewRequest` and `httptest.NewRecorder` — test handlers without a running server
- `*httptest.ResponseRecorder` satisfies `http.ResponseWriter` through implicit interface satisfaction
- Fake service pattern — configurable `err` field controls what the fake returns per test case
- Always add the service error test case — the 500 path is as important as the happy path
- Docker Compose healthcheck — `pg_isready` ensures DB is ready before the app connects

**Key Go concepts:**
- `r.URL.Query().Get("key")` — read query parameters from the request
- `httptest.NewRequest(method, url, body)` — fake HTTP request for tests
- `httptest.NewRecorder()` — fake response writer, records status and body in memory
- `rec.Code` — recorded status code
- `rec.Body.String()` — recorded response body as string
- `strings.NewReader(body)` — convert string to `io.Reader` for request body

**What confused me at first:**
- Why `*httptest.ResponseRecorder` can be passed where `http.ResponseWriter` is expected.
  `http.ResponseWriter` is an interface. `ResponseRecorder` implements all three methods.
  Same implicit satisfaction as `Notifier` on Day 2.

**What finally made it clear:**
- Everything in Go that accepts an interface can receive any type that satisfies it.
  The handler does not know what `w` actually is — real writer or test recorder.
  It just calls the methods. This is the same plug-and-socket pattern as dependency inversion.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Not testing the service error path | The 500 case is a real production path — must be tested |
| Missing `Content-Type` header in test requests | Tests pass but may miss real content-type validation bugs |
| Defaulting `status` silently without documentation | API callers don't know the default — document it in README |
| Not adding a healthcheck to Docker Compose | App may start before DB is ready — connection refused on startup |

---

## 3. Code Demo

### `internal/handler/event_test.go`

```go
package handler

import (
    "context"
    "errors"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type fakeEventService struct {
    err error
}

func (f fakeEventService) Create(ctx context.Context, eventType, payload string) error {
    return f.err
}

func (f fakeEventService) ListByStatus(ctx context.Context, status string) ([]domain.NotificationEvent, error) {
    return nil, nil
}

func TestCreateEvent(t *testing.T) {
    tests := []struct {
        name       string
        body       string
        mockErr    error
        wantStatus int
    }{
        {
            name:       "valid request",
            body:       `{"type":"email","payload":"hello"}`,
            mockErr:    nil,
            wantStatus: http.StatusAccepted,
        },
        {
            name:       "missing type",
            body:       `{"payload":"hello"}`,
            mockErr:    nil,
            wantStatus: http.StatusBadRequest,
        },
        {
            name:       "service error",
            body:       `{"type":"email","payload":"hello"}`,
            mockErr:    errors.New("database down"),
            wantStatus: http.StatusInternalServerError,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            svc := fakeEventService{err: tt.mockErr}
            h := NewEventHandler(svc)

            req := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(tt.body))
            req.Header.Set("Content-Type", "application/json")
            rec := httptest.NewRecorder()

            h.CreateEvent(rec, req)

            if rec.Code != tt.wantStatus {
                t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
            }
        })
    }
}
```

### `docker-compose.yml`

```yaml
version: "3.9"

services:
  db:
    image: postgres:16
    environment:
      POSTGRES_USER: notify
      POSTGRES_PASSWORD: notify
      POSTGRES_DB: notification_dispatcher
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U notify"]
      interval: 5s
      timeout: 5s
      retries: 5
```

### Manual test of `GET /events`

```bash
# list pending events
curl -s "http://localhost:8080/events?status=pending" | jq

# default — also returns pending
curl -s "http://localhost:8080/events" | jq
```

---

## 4. Interview Takeaway

**How Go handler testing works:**
`httptest.NewRequest` and `httptest.NewRecorder` let you test handlers in complete isolation —
no running server, no network, no database. The recorder implements `http.ResponseWriter`
through implicit interface satisfaction. The handler cannot tell the difference.

**Why fake services instead of real ones in handler tests:**
Handler tests should test HTTP concerns only — request parsing, validation, status codes, response format.
They should not test business logic or database queries — those have their own tests.
A fake service with a configurable error field covers all handler paths in milliseconds.

**The full testability pyramid:**
```
Handler tests   → fake service     → milliseconds, no DB
Service tests   → fake repository  → milliseconds, no DB
Repository tests → real database   → slower, integration tests only
```

**Why `httptest.ResponseRecorder` satisfies `http.ResponseWriter`:**
`http.ResponseWriter` is an interface with three methods.
`ResponseRecorder` implements all three — implicit satisfaction, same as any other interface in Go.
This is the same principle as `Notifier` on Day 2 applied to the HTTP layer.

---

## 5. Cleanup Notes

Day 14 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/handler/event_test.go` — real handler tests, grows through Week 4
- `docker-compose.yml` — project infrastructure, always committed

**Week 2 is complete.** All seven days committed. Week 3 begins the worker pool.