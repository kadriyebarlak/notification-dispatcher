# Day 16 — Notifier Interface & Fake Implementations

---

## 1. Original Lesson Explanation

### 1.1 Big picture

`EmailNotifier` and `WebhookNotifier` from Day 2 just print a line and return `nil`.
For Day 17's dispatcher to work properly and be testable, the notifiers need:

- Proper structured logging so you can see what is happening
- A configurable failure mode so you can test retry logic on Day 18
- A routing mechanism so the dispatcher knows which notifier handles which event type

---

### 1.2 The routing problem

The dispatcher receives a `NotificationEvent` with a `Type` field — `"email"` or `"webhook"`.
It needs to route it to the right notifier.

The cleanest Go pattern is a **registry map**:

```go
type NotifierRegistry struct {
    notifiers map[domain.EventType]domain.Notifier
}

func (r *NotifierRegistry) Get(eventType domain.EventType) (domain.Notifier, bool) {
    n, ok := r.notifiers[eventType]
    return n, ok
}
```

The dispatcher looks up the notifier by event type.
If none is registered, it handles the unknown type cleanly instead of panicking.

---

### 1.3 Configurable failure mode

For testing retry logic, notifiers need to be able to fail on demand:

```go
type FakeEmailNotifier struct {
    ShouldFail bool
}

func (n *FakeEmailNotifier) Send(ctx context.Context, event domain.NotificationEvent) error {
    if n.ShouldFail {
        return &domain.NotifyError{
            EventID: event.ID,
            Reason:  "simulated email failure",
        }
    }
    log.Printf("email sent for event %s type %s", event.ID, event.Type)
    return nil
}
```

`ShouldFail` is exported so tests can toggle it.
In production you swap this for a real email client — no other code changes.

---

### 1.4 EventType — named type with constants

Same pattern as `EventStatus` from Day 1:

```go
type EventType string

const (
    EventTypeEmail   EventType = "email"
    EventTypeWebhook EventType = "webhook"
)
```

Used as the map key in `NotifierRegistry` and as the `Type` field in `NotificationEvent`.

---

### 1.5 Go map concurrency rules

**Concurrent reads — safe.**
Multiple goroutines reading from a map simultaneously is fine. No locks needed.

**Concurrent read + write — data race.**
Go's map is not safe for concurrent writes. The runtime actively detects this and panics:
```
fatal error: concurrent map read and map write
```

**Your registry is safe** — built once at startup, never modified after.
All goroutines only read from it. Build at startup, treat as immutable, no locks needed.

**If the registry needed runtime modifications**, protect with `sync.RWMutex`:
```go
type NotifierRegistry struct {
    mu        sync.RWMutex
    notifiers map[domain.EventType]domain.Notifier
}

func (r *NotifierRegistry) Get(eventType domain.EventType) (domain.Notifier, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    n, ok := r.notifiers[eventType]
    return n, ok
}
```

`RLock` allows multiple concurrent readers. `Lock` blocks everyone.
**Do not add synchronisation before you have a real concurrent write problem.**

---

## 2. My Learning Summary

**What I learned:**
- `ShouldFail bool` field — configurable failure mode for testing retry logic
- Pointer receivers are now required — struct has mutable state (`ShouldFail`)
- Registry map built once at startup is safe for concurrent reads without locks
- Go map concurrent write is a runtime panic — not just undefined behaviour
- `sync.RWMutex` is the solution for maps that need runtime modifications — not needed here
- `_ = registry` suppresses "declared and not used" compiler error until Day 17 uses it

**Key Go concepts:**
- `map[KeyType]ValueType` — Go map, not safe for concurrent writes
- `sync.RWMutex` — allows multiple concurrent readers, exclusive writer
- `mu.RLock()` / `mu.RUnlock()` — read lock, multiple goroutines can hold simultaneously
- `mu.Lock()` / `mu.Unlock()` — write lock, exclusive
- Immutable-after-init pattern — build once, read-only forever, no locks needed

**What confused me at first:**
- Should `NotifierRegistry` have a mutex?
  No — the registry is built once and never modified.
  Adding a mutex would be solving a problem that does not exist.
  Add synchronisation only when you have concurrent writes.

**What finally made it clear:**
- The rule: add synchronisation when you have concurrent writes, not before.
  Premature locking adds complexity and overhead with no benefit.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Adding mutex to a read-only map | Over-engineering — solves a problem that does not exist |
| Concurrent writes to a map without mutex | Runtime panic — fatal error: concurrent map read and map write |
| Value receiver on a struct with mutable state | `ShouldFail` changes are lost — use pointer receiver |
| Not exporting `ShouldFail` | Tests in other packages cannot toggle it |
| Panicking on unknown event type in registry | Return `false` from `Get` and handle gracefully in dispatcher |

---

## 3. Code Demo

### `internal/notifier/email.go`

```go
package notifier

import (
    "context"
    "log"

    "github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type FakeEmailNotifier struct {
    ShouldFail bool
}

func (n *FakeEmailNotifier) Send(ctx context.Context, event domain.NotificationEvent) error {
    if n.ShouldFail {
        return &domain.NotifyError{
            EventID: event.ID,
            Reason:  "simulated email failure",
        }
    }
    log.Printf("email sent for event %s type %s", event.ID, event.Type)
    return nil
}
```

### `internal/notifier/webhook.go`

```go
package notifier

import (
    "context"
    "log"

    "github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type FakeWebhookNotifier struct {
    ShouldFail bool
}

func (n *FakeWebhookNotifier) Send(ctx context.Context, event domain.NotificationEvent) error {
    if n.ShouldFail {
        return &domain.NotifyError{
            EventID: event.ID,
            Reason:  "simulated webhook failure",
        }
    }
    log.Printf("webhook sent for event %s type %s", event.ID, event.Type)
    return nil
}
```

### `internal/notifier/registry.go`

```go
package notifier

import "github.com/kadriyebarlak/notification-dispatcher/internal/domain"

type NotifierRegistry struct {
    notifiers map[domain.EventType]domain.Notifier
}

func NewNotifierRegistry(notifiers map[domain.EventType]domain.Notifier) *NotifierRegistry {
    return &NotifierRegistry{notifiers: notifiers}
}

func (r *NotifierRegistry) Get(eventType domain.EventType) (domain.Notifier, bool) {
    n, ok := r.notifiers[eventType]
    return n, ok
}
```

### `cmd/server/main.go` — registry wiring

```go
registry := notifier.NewNotifierRegistry(map[domain.EventType]domain.Notifier{
    domain.EventTypeEmail:   &notifier.FakeEmailNotifier{},
    domain.EventTypeWebhook: &notifier.FakeWebhookNotifier{},
})
_ = registry // temporary — dispatcher uses it on Day 17
```

### Compile-time interface checks

```go
var _ domain.Notifier = (*notifier.FakeEmailNotifier)(nil)
var _ domain.Notifier = (*notifier.FakeWebhookNotifier)(nil)
```

---

## 4. Interview Takeaway

**Go map concurrency rules:**
Concurrent reads from a map are safe — no locks needed.
Concurrent writes — or a read concurrent with a write — cause a runtime panic.
Go's runtime actively detects this: `fatal error: concurrent map read and map write`.
For maps modified after initialisation, protect with `sync.RWMutex`.

**When NOT to add synchronisation:**
If a map is built once at startup and only read after that, it is safe without locks.
Adding a mutex to a read-only map is premature optimisation — adds complexity with no benefit.
Rule: add synchronisation when you have concurrent writes. Not before.

**Configurable failure mode pattern:**
Export a `ShouldFail bool` field on fake implementations.
Tests toggle it to simulate failure paths without touching production code.
In production, the fake is replaced by a real implementation — no other changes needed.

**Registry pattern:**
A `map[EventType]Notifier` built at startup and looked up at dispatch time.
`Get` returns `(Notifier, bool)` — the caller handles the unknown type case gracefully.
Never panic on missing keys — return a boolean and let the caller decide.

---

## 5. Cleanup Notes

Day 16 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/notifier/email.go` — updated with `ShouldFail` and pointer receiver
- `internal/notifier/webhook.go` — updated with `ShouldFail` and pointer receiver
- `internal/notifier/registry.go` — event type routing, used by dispatcher on Day 17

**Note:** `_ = registry` in `main.go` is temporary.
Remove it on Day 17 when the dispatcher is created and uses the registry.