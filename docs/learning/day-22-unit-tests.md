# Day 22 — Unit Tests with Mocked Interfaces

---

## 1. Original Lesson Explanation

### 1.1 Big picture

After Week 3, the most important business logic —
`EventService.Create` and `Dispatcher.Process` — has no tests.
Day 22 closes that gap.

The pattern is the same as Day 14's handler tests:
define a fake that implements the interface, inject it, test the behaviour.
No database, no network, no real notifiers.

---

### 1.2 What to test in `EventService`

`EventService.Create` does three things:
- Builds a `NotificationEvent` with `StatusPending` and `RetryCount: 0`
- Calls `repo.Insert`
- Returns the error from `repo.Insert` if any

Tests should verify all three paths:
- Happy path — `repo.Insert` succeeds
- Error path — `repo.Insert` fails, error is returned to caller

---

### 1.3 What to test in `Dispatcher.Process`

`Dispatcher.Process` has four paths:

| Path | Expected status | Expected retry count |
|---|---|---|
| No notifier for event type | `StatusDead` | unchanged |
| Notifier succeeds | `StatusDelivered` | unchanged |
| Notifier fails, below max retries | `StatusFailed` | incremented |
| Notifier fails, at max retries | `StatusDead` | incremented |

---

### 1.4 How many times does `Process` call `UpdateStatus`?

Every code path in `Process` calls `UpdateStatus` exactly once and then returns:

- No notifier → `UpdateStatus(StatusDead)` → return
- Notifier succeeds → `UpdateStatus(StatusDelivered)` → return
- Notifier fails below max → `UpdateStatus(StatusFailed)` → return
- Notifier fails at max → `UpdateStatus(StatusDead)` → return

Compare to `dispatch` — which sets `PROCESSING` before submitting.
`dispatch` sets the intermediate status. `Process` sets the final status.
Two separate responsibilities.

---

### 1.5 The fake repository pattern

Store what was called so tests can assert on it:

```go
type updateCall struct {
    id         string
    status     domain.EventStatus
    retryCount int
}

type fakeEventRepository struct {
    insertErr       error
    insertedEvents  []domain.NotificationEvent
    updatedStatuses []updateCall
}
```

Capturing `updateCall` as a struct — not just `[]domain.EventStatus` —
lets you assert on `id`, `status`, and `retryCount` together.
More thorough than asserting on status alone.

---

### 1.6 Passing `nil` for the worker pool in dispatcher tests

`NewDispatcher` takes a `*worker.WorkerPool`.
`Process` never touches the pool — it only uses `repo` and `registry`.
Passing `nil` is safe and keeps the test clean:

```go
dispatcher := NewDispatcher(
    repo,
    nil,       // pool — not used by Process
    tt.registry,
    time.Second,
    tt.maxRetries,
)
```

This is the correct approach for unit tests — only inject what the method under test actually uses.

---

### 1.7 `t.Fatalf` vs `t.Errorf`

```go
if len(repo.updatedStatuses) != 1 {
    t.Fatalf("got %d update calls, want 1", len(repo.updatedStatuses))
}
```

Use `t.Fatalf` when subsequent assertions would panic or give misleading results
if the current check fails. Use `t.Errorf` when the test can continue safely.

Here, if `updatedStatuses` is empty, `repo.updatedStatuses[0]` would panic.
`Fatalf` stops the test immediately with a clear message.

---

## 2. My Learning Summary

**What I learned:**
- Service tests verify the event is built correctly — not just that Insert was called
- Dispatcher tests cover all four `Process` paths — every meaningful code path has a test
- `updateCall` struct captures all three fields — more thorough than storing just the status
- `nil` for unused constructor parameters — clean unit test pattern
- `t.Fatalf` stops immediately when later assertions would be unsafe
- `Process` always calls `UpdateStatus` exactly once — one path, one final status

**Key Go concepts:**
- Fake struct with configurable error fields — `insertErr error`, `shouldFail bool`
- Recording calls for assertion — `insertedEvents`, `updatedStatuses` slices
- `t.Fatalf` vs `t.Errorf` — stop vs continue on failure
- `errors.Is` for error comparison — works through wrapping
- Injecting `nil` for unused dependencies — safe when the method doesn't touch them

**What was done better than the lesson suggested:**
- `updateCall` struct instead of `[]domain.EventStatus` — captures id, status, and retryCount
  together. This catches bugs where the wrong event ID is updated, not just the wrong status.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Only asserting that Insert was called | Misses bugs in how the event is built |
| Using `t.Errorf` before slice index access | Test panics if slice is empty |
| Storing only status in fake — not id and retryCount | Misses bugs in wrong ID updated or wrong count |
| Using a real worker pool in dispatcher unit tests | Starts goroutines — test becomes slow and flaky |
| Not testing all code paths | A path without a test is a path without a guarantee |

---

## 3. Code Demo

### `internal/service/event_service_test.go`

```go
package service

import (
    "context"
    "errors"
    "testing"

    "github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type fakeEventRepository struct {
    insertErr      error
    insertedEvents []domain.NotificationEvent
    updatedStatuses []domain.EventStatus
}

func (f *fakeEventRepository) Insert(ctx context.Context, event domain.NotificationEvent) error {
    f.insertedEvents = append(f.insertedEvents, event)
    return f.insertErr
}

func (f *fakeEventRepository) UpdateStatus(ctx context.Context, id string, status domain.EventStatus, retryCount int) error {
    f.updatedStatuses = append(f.updatedStatuses, status)
    return f.updateStatusErr
}

func (f *fakeEventRepository) FindByStatus(ctx context.Context, status domain.EventStatus) ([]domain.NotificationEvent, error) {
    return nil, nil
}

func (f *fakeEventRepository) FindByStatuses(ctx context.Context, statuses ...domain.EventStatus) ([]domain.NotificationEvent, error) {
    return nil, nil
}

func TestEventService_Create(t *testing.T) {
    insertErr := errors.New("insert failed")

    tests := []struct {
        name          string
        eventType     string
        payload       string
        insertErr     error
        wantErr       error
        wantInsertLen int
    }{
        {
            name:          "valid event inserted successfully",
            eventType:     "email",
            payload:       "hello",
            wantInsertLen: 1,
        },
        {
            name:          "returns error when insert fails",
            eventType:     "email",
            payload:       "hello",
            insertErr:     insertErr,
            wantErr:       insertErr,
            wantInsertLen: 1,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := &fakeEventRepository{insertErr: tt.insertErr}
            svc := NewEventService(repo)

            err := svc.Create(context.Background(), tt.eventType, tt.payload)

            if !errors.Is(err, tt.wantErr) {
                t.Errorf("got error %v, want %v", err, tt.wantErr)
            }

            if len(repo.insertedEvents) != tt.wantInsertLen {
                t.Errorf("got %d inserted events, want %d", len(repo.insertedEvents), tt.wantInsertLen)
            }

            if len(repo.insertedEvents) == 0 {
                return
            }

            got := repo.insertedEvents[0]

            if got.Type != domain.EventType(tt.eventType) {
                t.Errorf("got type %q, want %q", got.Type, tt.eventType)
            }
            if got.Payload != tt.payload {
                t.Errorf("got payload %q, want %q", got.Payload, tt.payload)
            }
            if got.Status != domain.StatusPending {
                t.Errorf("got status %q, want %q", got.Status, domain.StatusPending)
            }
            if got.RetryCount != 0 {
                t.Errorf("got retry count %d, want 0", got.RetryCount)
            }
        })
    }
}
```

### `internal/dispatcher/dispatcher_test.go`

```go
package dispatcher

import (
    "context"
    "errors"
    "testing"
    "time"

    "github.com/kadriyebarlak/notification-dispatcher/internal/domain"
    "github.com/kadriyebarlak/notification-dispatcher/internal/notifier"
)

type updateCall struct {
    id         string
    status     domain.EventStatus
    retryCount int
}

type fakeEventRepository struct {
    updatedStatuses []updateCall
}

func (f *fakeEventRepository) Insert(ctx context.Context, event domain.NotificationEvent) error { return nil }
func (f *fakeEventRepository) FindByStatus(ctx context.Context, status domain.EventStatus) ([]domain.NotificationEvent, error) { return nil, nil }
func (f *fakeEventRepository) FindByStatuses(ctx context.Context, statuses ...domain.EventStatus) ([]domain.NotificationEvent, error) { return nil, nil }

func (f *fakeEventRepository) UpdateStatus(ctx context.Context, id string, status domain.EventStatus, retryCount int) error {
    f.updatedStatuses = append(f.updatedStatuses, updateCall{id: id, status: status, retryCount: retryCount})
    return nil
}

type fakeNotifier struct{ shouldFail bool }

func (f *fakeNotifier) Send(ctx context.Context, event domain.NotificationEvent) error {
    if f.shouldFail {
        return errors.New("simulated failure")
    }
    return nil
}

func TestDispatcher_Process(t *testing.T) {
    tests := []struct {
        name           string
        event          domain.NotificationEvent
        registry       *notifier.NotifierRegistry
        maxRetries     int
        wantStatus     domain.EventStatus
        wantRetryCount int
    }{
        {
            name:           "no notifier registered marks event as dead",
            event:          domain.NotificationEvent{ID: "evt-001", Type: domain.EventTypeEmail, RetryCount: 0},
            registry:       notifier.NewNotifierRegistry(map[domain.EventType]domain.Notifier{}),
            maxRetries:     3,
            wantStatus:     domain.StatusDead,
            wantRetryCount: 0,
        },
        {
            name:  "notifier succeeds marks event as delivered",
            event: domain.NotificationEvent{ID: "evt-002", Type: domain.EventTypeEmail, RetryCount: 0},
            registry: notifier.NewNotifierRegistry(map[domain.EventType]domain.Notifier{
                domain.EventTypeEmail: &fakeNotifier{shouldFail: false},
            }),
            maxRetries:     3,
            wantStatus:     domain.StatusDelivered,
            wantRetryCount: 0,
        },
        {
            name:  "notifier fails below max retries marks event as failed",
            event: domain.NotificationEvent{ID: "evt-003", Type: domain.EventTypeEmail, RetryCount: 1},
            registry: notifier.NewNotifierRegistry(map[domain.EventType]domain.Notifier{
                domain.EventTypeEmail: &fakeNotifier{shouldFail: true},
            }),
            maxRetries:     3,
            wantStatus:     domain.StatusFailed,
            wantRetryCount: 2,
        },
        {
            name:  "notifier fails at max retries marks event as dead",
            event: domain.NotificationEvent{ID: "evt-004", Type: domain.EventTypeEmail, RetryCount: 2},
            registry: notifier.NewNotifierRegistry(map[domain.EventType]domain.Notifier{
                domain.EventTypeEmail: &fakeNotifier{shouldFail: true},
            }),
            maxRetries:     3,
            wantStatus:     domain.StatusDead,
            wantRetryCount: 3,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            repo := &fakeEventRepository{}
            disp := NewDispatcher(repo, nil, tt.registry, time.Second, tt.maxRetries)

            disp.Process(context.Background(), tt.event)

            if len(repo.updatedStatuses) != 1 {
                t.Fatalf("got %d update calls, want 1", len(repo.updatedStatuses))
            }

            got := repo.updatedStatuses[0]

            if got.id != tt.event.ID {
                t.Errorf("got id %q, want %q", got.id, tt.event.ID)
            }
            if got.status != tt.wantStatus {
                t.Errorf("got status %q, want %q", got.status, tt.wantStatus)
            }
            if got.retryCount != tt.wantRetryCount {
                t.Errorf("got retry count %d, want %d", got.retryCount, tt.wantRetryCount)
            }
        })
    }
}
```

---

## 4. Interview Takeaway

**Why unit tests use fakes instead of real dependencies:**
Unit tests should test one thing in isolation — the logic of the function under test.
Real dependencies (database, notifiers) make tests slow, fragile, and hard to control.
Fakes implement the same interface and return exactly what the test needs.

**The fake repository pattern:**
Store calls in slices — `insertedEvents`, `updatedStatuses`.
Assert on what was stored — not just that the method was called, but how it was called.
This catches bugs in how data is built, not just whether a method was invoked.

**`t.Fatalf` vs `t.Errorf`:**
`t.Errorf` — marks the test as failed but continues running.
`t.Fatalf` — marks the test as failed and stops immediately.
Use `Fatalf` when subsequent assertions would panic or give misleading results.

**Passing `nil` for unused dependencies:**
In unit tests, only inject what the method under test actually uses.
If `Process` never touches the worker pool, pass `nil`.
If a future change makes `Process` use the pool, the test will panic — which is the correct signal to update the test.

---

## 5. Cleanup Notes

Day 22 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/service/event_service_test.go` — service layer tests
- `internal/dispatcher/dispatcher_test.go` — dispatcher Process tests