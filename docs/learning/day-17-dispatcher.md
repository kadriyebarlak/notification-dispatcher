# Day 17 — Dispatcher: Connecting Pool to Notifiers

---

## 1. Original Lesson Explanation

### 1.1 Big picture

After Day 16, all the pieces exist:
- A worker pool that processes jobs concurrently
- A notifier registry that routes event types to notifiers
- A repository that reads and updates events in PostgreSQL

The dispatcher is the glue. It has one job: **poll the database for pending events
and feed them into the worker pool.**

---

### 1.2 The dispatcher's responsibility

```
Dispatcher loop (every N seconds):
  1. Query DB for PENDING events
  2. For each event:
     a. Update status to PROCESSING
     b. Submit to worker pool

Worker processes event:
  1. Look up notifier in registry by event type
  2. Call notifier.Send(ctx, event)
  3. If success → update status to DELIVERED
  4. If failure → update status to FAILED (retry logic on Day 18)
```

The dispatcher loop runs in its own goroutine. Workers run in their own goroutines.
The HTTP server runs in the main goroutine. All three run concurrently —
and all three stop cleanly when the context is cancelled.

---

### 1.3 The dispatcher struct

```go
type Dispatcher struct {
    repo     domain.EventRepository
    pool     *worker.WorkerPool
    registry *notifier.NotifierRegistry
    interval time.Duration
}
```

---

### 1.4 The polling loop

```go
func (d *Dispatcher) Start(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(d.interval)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                d.dispatch(ctx)
            case <-ctx.Done():
                return
            }
        }
    }()
}
```

`time.NewTicker` fires every `d.interval`.
`defer ticker.Stop()` is mandatory — without it the ticker goroutine leaks memory forever.
On context cancellation the loop exits cleanly.

> **Türkçe özet:** Dispatcher her N saniyede bir DB'yi kontrol eder.
> Yeni PENDING event varsa worker pool'a gönderir.
> Context iptal olunca durur. `ticker.Stop()` unutulursa kaynak sızıntısı olur.

---

### 1.5 The dispatch function

```go
func (d *Dispatcher) dispatch(ctx context.Context) {
    events, err := d.repo.FindByStatus(ctx, domain.StatusPending)
    if err != nil {
        log.Printf("dispatcher: failed to fetch events: %v", err)
        return
    }

    for _, event := range events {
        if err := d.repo.UpdateStatus(ctx, event.ID, domain.StatusProcessing, event.RetryCount); err != nil {
            log.Printf("dispatcher: failed to update status: %v", err)
            continue  // one failure does not block other events
        }
        d.pool.Submit(event)
    }
}
```

`continue` on `UpdateStatus` failure — one event failing should not block all other
pending events in the same tick. Each event is handled independently.

---

### 1.6 Why PROCESSING status must be set before Submit

**If you submitted to the pool first and updated status after:**

```
Tick 1: event fetched (status = PENDING), submitted to pool
Tick 2 fires before worker finishes: event fetched again (still PENDING), submitted again
Result: same event processed twice by two different workers simultaneously
```

Two workers calling `notifier.Send` for the same event — duplicate emails, duplicate webhooks.
Users receive the same notification twice. This is a real production bug.

**Setting PROCESSING first acts as a lock at the database level.**
The next tick's `FindByStatus` query only fetches `PENDING` events —
it never sees events already marked `PROCESSING`.

---

### 1.7 Optimistic locking and SELECT FOR UPDATE SKIP LOCKED

Setting `PROCESSING` before `Submit` is **application-level optimistic locking**.
It works correctly for a single instance of the service.

**The limitation:** if you run two instances of the service simultaneously
(horizontal scaling), both instances could fetch the same `PENDING` event
in the same millisecond before either has a chance to update it to `PROCESSING`.
Result: duplicate processing across instances.

**The production solution:** `SELECT FOR UPDATE SKIP LOCKED` at the database level:

```sql
SELECT id, type, payload, status, retry_count
FROM events
WHERE status = 'pending'
FOR UPDATE SKIP LOCKED
LIMIT 10;
```

- `FOR UPDATE` — locks the selected rows at the DB level, no other transaction can touch them
- `SKIP LOCKED` — other instances skip already-locked rows instead of waiting

This is the standard pattern for distributed job queues backed by PostgreSQL.
With this query, multiple instances of the service can safely run simultaneously —
each instance gets a unique set of events to process, guaranteed by the database.

For the MVP with a single instance, the application-level `PROCESSING` status is sufficient.
Mention `SELECT FOR UPDATE SKIP LOCKED` in interviews when asked about scaling.

---

### 1.8 The process function

```go
func (d *Dispatcher) Process(ctx context.Context, event domain.NotificationEvent) {
    notifier, ok := d.registry.Get(event.Type)
    if !ok {
        log.Printf("dispatcher: no notifier for event type %s", event.Type)
        d.repo.UpdateStatus(ctx, event.ID, domain.StatusFailed, event.RetryCount)
        return
    }

    if err := notifier.Send(ctx, event); err != nil {
        log.Printf("dispatcher: failed to send event %s: %v", event.ID, err)
        d.repo.UpdateStatus(ctx, event.ID, domain.StatusFailed, event.RetryCount)
        return
    }

    d.repo.UpdateStatus(ctx, event.ID, domain.StatusDelivered, event.RetryCount)
}
```

Unknown event type → `StatusFailed`, not a panic. Always handle unknown cases gracefully.

---

### 1.9 Wiring order in `main.go`

```go
workerPool.Start(ctx, disp.Process)  // start workers first
disp.Start(ctx)                       // then start polling
```

Starting the dispatcher before the workers would submit jobs into a pool
with no workers running yet. Workers must be ready before the dispatcher
starts feeding them jobs.

---

## 2. My Learning Summary

**What I learned:**
- Dispatcher polls DB every N seconds using `time.NewTicker`
- `defer ticker.Stop()` is mandatory — without it the ticker goroutine leaks
- `PROCESSING` status set before `Submit` prevents duplicate processing on the next tick
- `continue` on single event failure — one bad event should not block others
- Unknown event type → `StatusFailed`, never panic
- Workers must start before the dispatcher — order matters in `main.go`
- Application-level locking works for single instance; `SELECT FOR UPDATE SKIP LOCKED` for multi-instance

**Key Go concepts:**
- `time.NewTicker(d)` — fires on a channel every duration `d`
- `defer ticker.Stop()` — releases ticker resources, prevents goroutine leak
- `ticker.C` — the channel that receives on each tick
- `select` with `ticker.C` and `ctx.Done()` — poll or stop cleanly
- `continue` in error handling loop — skip one, process the rest

**What confused me at first:**
- Why `PROCESSING` before `Submit` and not after?
  If status is updated after submit, the next tick can fetch the same event again
  before the worker finishes — duplicate processing.

**What finally made it clear:**
- The database status is the lock. Setting it to `PROCESSING` before the worker
  starts is what prevents the next tick from seeing the event as `PENDING` again.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Forgetting `defer ticker.Stop()` | Ticker goroutine leaks — memory grows over time |
| Submitting to pool before updating status | Duplicate events on next tick — users get notified twice |
| Using `return` instead of `continue` on single event error | One bad event blocks all others in the same tick |
| Starting dispatcher before worker pool | Jobs submitted with no workers to process them |
| Panicking on unknown event type | Crashes the worker — use StatusFailed and log instead |

---

## 3. Code Demo

### `internal/dispatcher/dispatcher.go`

```go
package dispatcher

import (
    "context"
    "log"
    "time"

    "github.com/kadriyebarlak/notification-dispatcher/internal/domain"
    "github.com/kadriyebarlak/notification-dispatcher/internal/notifier"
    "github.com/kadriyebarlak/notification-dispatcher/internal/worker"
)

type Dispatcher struct {
    repo     domain.EventRepository
    pool     *worker.WorkerPool
    registry *notifier.NotifierRegistry
    interval time.Duration
}

func NewDispatcher(
    repo domain.EventRepository,
    pool *worker.WorkerPool,
    registry *notifier.NotifierRegistry,
    interval time.Duration,
) *Dispatcher {
    return &Dispatcher{
        repo:     repo,
        pool:     pool,
        registry: registry,
        interval: interval,
    }
}

func (d *Dispatcher) Start(ctx context.Context) {
    go func() {
        ticker := time.NewTicker(d.interval)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                d.dispatch(ctx)
            case <-ctx.Done():
                return
            }
        }
    }()
}

func (d *Dispatcher) dispatch(ctx context.Context) {
    events, err := d.repo.FindByStatus(ctx, domain.StatusPending)
    if err != nil {
        log.Printf("dispatcher: failed to fetch events: %v", err)
        return
    }

    for _, event := range events {
        if err := d.repo.UpdateStatus(ctx, event.ID, domain.StatusProcessing, event.RetryCount); err != nil {
            log.Printf("dispatcher: failed to update status: %v", err)
            continue
        }
        d.pool.Submit(event)
    }
}

func (d *Dispatcher) Process(ctx context.Context, event domain.NotificationEvent) {
    notifier, ok := d.registry.Get(event.Type)
    if !ok {
        log.Printf("dispatcher: no notifier for event type %s", event.Type)
        d.repo.UpdateStatus(ctx, event.ID, domain.StatusFailed, event.RetryCount)
        return
    }

    if err := notifier.Send(ctx, event); err != nil {
        log.Printf("dispatcher: failed to send event %s: %v", event.ID, err)
        d.repo.UpdateStatus(ctx, event.ID, domain.StatusFailed, event.RetryCount)
        return
    }

    d.repo.UpdateStatus(ctx, event.ID, domain.StatusDelivered, event.RetryCount)
}
```

### `cmd/server/main.go` — full wiring

```go
registry := notifier.NewNotifierRegistry(map[domain.EventType]domain.Notifier{
    domain.EventTypeEmail:   &notifier.FakeEmailNotifier{},
    domain.EventTypeWebhook: &notifier.FakeWebhookNotifier{},
})

workerPool := worker.NewWorkerPool(3)
disp := dispatcher.NewDispatcher(eventRepository, workerPool, registry, 5*time.Second)

workerPool.Start(ctx, disp.Process)  // workers first
disp.Start(ctx)                       // dispatcher second
```

### End-to-end test

```bash
# submit event
curl -s -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{"type":"email","payload":"hello world"}' | jq

# wait 5 seconds, check status
docker exec -it notification-db psql -U notify \
  -d notification_dispatcher \
  -c "SELECT id, type, status FROM events;"

# expected: status = delivered
# expected log line: email sent for event evt-xxx type email
```

---

## 4. Interview Takeaway

**What the dispatcher does:**
Polls the database every N seconds for `PENDING` events.
Updates each to `PROCESSING` before submitting to the worker pool.
Workers call the appropriate notifier and update the final status.

**Why PROCESSING before Submit — application-level locking:**
Setting `PROCESSING` before submitting to the pool acts as a database-level lock.
The next tick's query only fetches `PENDING` events — it never sees events already
marked `PROCESSING`. Without this, the same event is processed multiple times.

**Scaling to multiple instances — SELECT FOR UPDATE SKIP LOCKED:**
Application-level locking only works for a single instance.
For horizontal scaling, use `SELECT FOR UPDATE SKIP LOCKED` in the query.
This locks rows at the database level — multiple instances each get unique events,
guaranteed by PostgreSQL. No two instances can process the same event simultaneously.

**Why `continue` instead of `return` in the dispatch loop:**
Each event in the loop is independent. One event failing to update its status
should not prevent other events from being processed in the same tick.
`continue` skips the failed event. `return` would abandon all remaining events.

**Ticker and goroutine leak prevention:**
`time.NewTicker` starts an internal goroutine. If you never call `Stop()`,
that goroutine runs forever — a goroutine leak. Always `defer ticker.Stop()`
immediately after creating the ticker.

---

## 5. Cleanup Notes

Day 17 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/dispatcher/dispatcher.go` — core dispatch engine
- `cmd/server/main.go` — updated with full wiring, `_ = registry` removed

**Note:** Retry logic is not yet implemented in `Process`.
A failed notification sets status to `FAILED` permanently.
Retry logic with `retry_count` is added on Day 18.