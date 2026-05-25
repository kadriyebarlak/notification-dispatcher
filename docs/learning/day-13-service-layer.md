# Day 13 — Service Layer & Dependency Injection

---

## 1. Original Lesson Explanation

### 1.1 Big picture

After Day 12, the handler decodes requests and returns 202. The repository exists but nothing
calls it. Day 13 connects all the layers.

In Spring Boot this wiring is invisible — `@Autowired`, `@Service`, `@Repository` and the
framework injects everything. In Go **you wire it yourself in `main.go`**. No magic, no container,
no classpath scanning. Just constructor functions called in the right order.

---

### 1.2 The layered structure

```
HTTP Request
     ↓
EventHandler        — receives HTTP, calls service
     ↓
EventService        — business logic, calls repository
     ↓
EventRepository     — talks to PostgreSQL
```

Each layer depends on an **interface**, not a concrete type.
This means each layer can be tested in isolation with a mock — no real database needed.

> **Türkçe özet:** Service katmanı handler ile repository arasındaki iş mantığını tutar.
> Handler HTTP bilir, repository SQL bilir, service ikisini de bilmez — sadece domain'i bilir.
> Her katman interface'e bağlıdır, somut tipe değil.

---

### 1.3 The service layer

`EventService` holds business logic that the handler should not know about:
- Setting initial status to `StatusPending`
- Setting `RetryCount` to 0
- Any future rules — rate limiting, deduplication, routing

```go
type EventService struct {
    repo domain.EventRepository  // interface — not *PostgresEventRepository
}

func NewEventService(repo domain.EventRepository) *EventService {
    return &EventService{repo: repo}
}

func (s *EventService) Create(ctx context.Context, eventType, payload string) error {
    event := domain.NotificationEvent{
        Type:       domain.EventType(eventType),
        Payload:    payload,
        Status:     domain.StatusPending,
        RetryCount: 0,
    }
    return s.repo.Insert(ctx, event)
}
```

---

### 1.4 Manual dependency injection in `main.go`

In Spring, the container builds the dependency graph. In Go, you do it yourself:

```go
// 1. connect to database
pool, err := pgxpool.New(ctx, dbURL)
if err := pool.Ping(ctx); err != nil {
    log.Fatal("cannot connect to database:", err)
}

// 2. create repository — depends on pool
repo := repository.NewPostgresEventRepository(pool)

// 3. create service — depends on repository interface
svc := service.NewEventService(repo)

// 4. create handler — depends on service interface
h := handler.NewEventHandler(svc)

// 5. wire handler into router
r.Post("/events", h.CreateEvent)
```

The entire dependency graph is visible in three lines. You can read `main.go` and understand
the wiring at a glance. No framework, no magic.

---

### 1.5 The handler's own interface

The handler package defines its own `EventService` interface:

```go
// defined in handler package — not imported from service package
type EventService interface {
    Create(ctx context.Context, eventType, payload string) error
}
```

This means the handler package does not import the service package at all.

**Why this matters for testing:**

Go can only mock interfaces — not concrete types. Java has Mockito which can mock concrete
classes using proxies. Go has no such mechanism.

If the handler imported `*service.EventService` directly:

```go
// WITHOUT interface — handler test requires a real database
type EventHandler struct {
    service *service.EventService  // concrete — cannot be mocked
}
```

Every handler test would need a real `EventService`, which needs a real `PostgresEventRepository`,
which needs a running PostgreSQL database. Slow, fragile, painful in CI.

With the interface, tests use a fake:

```go
// WITH interface — handler test needs no database
type fakeEventService struct{}

func (f *fakeEventService) Create(ctx context.Context, eventType, payload string) error {
    return nil
}

handler := NewEventHandler(&fakeEventService{})
```

---

### 1.6 Why every layer uses an interface

The same logic applies at every level:

```
EventHandler  → EventService interface    → testable without real service
EventService  → EventRepository interface → testable without real database
PostgresRepo  → pgxpool.Pool              → tested with real DB (integration test)
```

Each layer only knows the interface below it.
Each layer can be tested without the layers below it.
This is not just organisation — it is **testability at every level**.

---

### 1.7 Dependency inversion — simply

Forget the fancy name. Think of it as a plug and socket.

**Without inversion:** the TV is hardwired to one specific DVD player.
You cannot swap it for testing or production changes.

**With inversion:** the TV has an HDMI port — a standard interface.
Anything that fits the port works: DVD player, Blu-ray, fake device for testing.

```go
// production
handler := NewEventHandler(realService)

// test
handler := NewEventHandler(fakeService)
```

The handler code does not change. Only what you plug in changes.

---

## 2. My Learning Summary

**What I learned:**
- Service layer holds business rules — status, retry count, future routing logic
- Manual DI in `main.go`: constructors called in order, dependency graph visible at a glance
- Handler defines its own `EventService` interface — consumer owns the contract
- Go cannot mock concrete types — only interfaces — so interfaces are required for testability
- `pool.Ping(ctx)` after `pgxpool.New` — confirm DB is reachable before serving traffic
- `r.Context()` passed to `h.service.Create` — request context flows through all layers

**Key Go concepts:**
- Constructor chain: `NewPostgresEventRepository → NewEventService → NewEventHandler`
- Consumer-side interface: handler defines what it needs, service satisfies it
- `pool.Ping(ctx)` — database health check on startup
- Pointer receiver on handler: `(h *EventHandler)` — struct now holds a dependency
- `r.Context()` — HTTP request context propagated into service and repository calls

**What confused me at first:**
- Why define an `EventService` interface in the handler package instead of importing
  `*service.EventService` directly?
  In Java, Mockito can mock concrete classes. In Go, only interfaces can be mocked.
  Without the interface, every handler test requires a real running database.

**What finally made it clear:**
- The testability argument. The interface is not about philosophy —
  it is about being able to write fast, isolated tests without a database.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Depending on concrete types instead of interfaces | Cannot mock in tests — requires real dependencies |
| Skipping `pool.Ping` on startup | Server starts but every request fails if DB is down |
| Putting business logic in the handler | Handler becomes untestable and hard to reuse |
| Using `context.Background()` in handler instead of `r.Context()` | Breaks request timeout and cancellation chain |
| Wiring dependencies inside handlers | Tight coupling — dependencies cannot be swapped |

---

## 3. Code Demo

### `internal/service/event_service.go`

```go
package service

import (
    "context"

    "github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type EventService struct {
    repo domain.EventRepository
}

func NewEventService(repo domain.EventRepository) *EventService {
    return &EventService{repo: repo}
}

func (s *EventService) Create(ctx context.Context, eventType, payload string) error {
    event := domain.NotificationEvent{
        Type:       domain.EventType(eventType),
        Payload:    payload,
        Status:     domain.StatusPending,
        RetryCount: 0,
    }
    return s.repo.Insert(ctx, event)
}
```

### `internal/handler/event.go` — with interface and constructor

```go
package handler

import (
    "context"
    "encoding/json"
    "net/http"
)

type EventService interface {
    Create(ctx context.Context, eventType, payload string) error
}

type EventHandler struct {
    service EventService
}

func NewEventHandler(service EventService) *EventHandler {
    return &EventHandler{service: service}
}

type CreateEventRequest struct {
    Type    string `json:"type"`
    Payload string `json:"payload"`
}

func (req CreateEventRequest) validate() []string {
    var errs []string
    if req.Type == "" {
        errs = append(errs, "type is required")
    }
    if req.Payload == "" {
        errs = append(errs, "payload is required")
    }
    return errs
}

func (h *EventHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
    var req CreateEventRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if errs := req.validate(); len(errs) > 0 {
        writeJSON(w, http.StatusBadRequest, map[string][]string{
            "errors": errs,
        })
        return
    }
    if err := h.service.Create(r.Context(), req.Type, req.Payload); err != nil {
        writeError(w, http.StatusInternalServerError, "failed to create event")
        return
    }
    writeJSON(w, http.StatusAccepted, map[string]string{
        "status": "accepted",
    })
}
```

### `cmd/server/main.go` — full dependency wiring

```go
ctx := context.Background()

pool, err := pgxpool.New(ctx, dbURL)
if err != nil {
    log.Fatal(err)
}
defer pool.Close()

if err := pool.Ping(ctx); err != nil {
    log.Fatal("cannot connect to database:", err)
}

eventRepository := repository.NewPostgresEventRepository(pool)
eventService    := service.NewEventService(eventRepository)
eventHandler    := handler.NewEventHandler(eventService)

r := chi.NewRouter()
r.Use(handler.LoggingMiddleware)
r.Use(handler.TimeoutMiddleware(5 * time.Second))
r.Post("/events", eventHandler.CreateEvent)
```

### End-to-end test

```bash
# insert an event
curl -s -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{"type":"email","payload":"hello world"}' | jq

# verify in database
docker exec -it notification-db psql -U notify \
  -d notification_dispatcher \
  -c "SELECT * FROM events;"
```

---

## 4. Interview Takeaway

**What the service layer is for:**
Business logic that does not belong in the handler (HTTP concerns) or repository (SQL concerns).
Initial status, retry count, routing rules, deduplication — all go in the service.

**Why Go uses manual DI instead of a framework:**
Go's simplicity philosophy. The dependency graph is visible in `main.go` — three constructor calls.
No classpath scanning, no reflection, no container startup time. You read the code and see exactly
what depends on what.

**Why interfaces enable testability:**
Go cannot mock concrete types — only interfaces.
If a handler depends on `*service.EventService`, every handler test requires a real database.
If it depends on an `EventService` interface, tests use a fake implementation — fast and isolated.

**The layered testability rule:**
```
Handler tests  → mock the service interface   → no real service needed
Service tests  → mock the repository interface → no real database needed
Repository tests → use real database           → integration tests only
```

---

## 5. Cleanup Notes

Day 13 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/service/event_service.go` — business logic layer
- `internal/handler/event.go` — updated with interface, constructor, service call
- `cmd/server/main.go` — full dependency wiring with pool.Ping