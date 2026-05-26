# Day 15 — Worker Pool Pattern in Go

---

## 1. Original Lesson Explanation

### 1.1 Big picture

After Week 2, events are stored in the database with status `pending` but nothing processes them.
A **worker pool** is the background engine that picks up pending events and dispatches them
through the right notifier.

In Java you would use `ThreadPoolExecutor` or Spring's `@Async` with a task executor.
Go has no such framework component. You build it yourself with goroutines and channels —
and it ends up being cleaner and more explicit than the Java version.

---

### 1.2 Why a worker pool

Processing events directly in the HTTP handler is wrong for three reasons:

- The HTTP handler would be slow — the client waits for the notification to be delivered
- If the notifier fails, the HTTP request fails — these are separate concerns
- You cannot control concurrency — every request spawns unbounded work

A worker pool solves all three:
- HTTP handler stores the event and returns 202 immediately — fast
- Workers process events independently — failures don't affect the API
- You control exactly how many workers run concurrently

> **Türkçe özet:** HTTP handler sadece event'i kaydeder ve hemen döner.
> Worker pool arka planda çalışır, event'leri alır ve notifier'lara iletir.
> İkisi birbirinden bağımsızdır — bu Go'nun concurrency modelinin tam kullanım şeklidir.

---

### 1.3 The shape of a worker pool

```
                    ┌─────────────┐
                    │  Dispatcher │  ← polls DB for pending events
                    └──────┬──────┘
                           │ jobs channel
              ┌────────────┼────────────┐
              ↓            ↓            ↓
         [Worker 1]   [Worker 2]   [Worker 3]   ← goroutines
              ↓            ↓            ↓
         [Notifier]   [Notifier]   [Notifier]   ← email, webhook
```

- One dispatcher goroutine polls the DB for `PENDING` events and sends them into a buffered channel
- N worker goroutines read from the channel and call the appropriate notifier
- A `sync.WaitGroup` waits for all workers to finish on shutdown
- A `context.Context` signals everyone to stop

This is the producer/consumer pattern from Day 5 applied to the real project.

---

### 1.4 The `WorkerPool` struct

```go
type WorkerPool struct {
    jobs        chan domain.NotificationEvent
    workerCount int
    wg          sync.WaitGroup
}

func NewWorkerPool(workerCount int) *WorkerPool {
    return &WorkerPool{
        jobs:        make(chan domain.NotificationEvent, workerCount*2),
        workerCount: workerCount,
    }
}
```

Buffer size `workerCount * 2` — enough buffer for workers to stay busy without
unbounded memory growth. A common production convention.

---

### 1.5 Start, Stop, Submit

```go
func (p *WorkerPool) Start(ctx context.Context, process func(ctx context.Context, event domain.NotificationEvent)) {
    for i := 0; i < p.workerCount; i++ {
        p.wg.Add(1)
        go func() {
            defer p.wg.Done()
            for {
                select {
                case event, ok := <-p.jobs:
                    if !ok {
                        return // channel closed — exit cleanly
                    }
                    process(ctx, event)
                case <-ctx.Done():
                    return // context cancelled — exit cleanly
                }
            }
        }()
    }
}

func (p *WorkerPool) Stop() {
    close(p.jobs) // signal workers: no more jobs
    p.wg.Wait()   // wait for all workers to finish
}

func (p *WorkerPool) Submit(event domain.NotificationEvent) {
    p.jobs <- event
}
```

`process` is a function parameter — the pool does not know what processing means.
It just distributes. This keeps the pool generic and reusable.

---

### 1.6 Why `close` must come before `Wait`

Workers block on `select` when there are no jobs — waiting for either a job or `ctx.Done()`.

If you call `wg.Wait()` before `close(p.jobs)`:
- Workers are blocked on `select` — no signal arrives
- `wg.Wait()` blocks waiting for workers that are themselves blocked
- **Deadlock** — the program hangs permanently

`close(p.jobs)` unblocks workers — they see `ok == false` and return, calling `wg.Done()`.
Only then can `wg.Wait()` return.

**Rule: signal first, wait second. Always.**

---

### 1.7 Testing concurrent code safely

Multiple workers incrementing a shared counter is a race condition without protection:

```go
// WRONG — data race
processedCount++

// CORRECT — mutex protected
mu.Lock()
processedCount++
mu.Unlock()
```

Always run concurrent tests with the race detector:
```bash
go test -race ./internal/worker/...
```

---

## 2. My Learning Summary

**What I learned:**
- Worker pool decouples HTTP handling from event processing — HTTP returns 202 immediately
- The pool is generic — `process` is a function parameter, not hardcoded business logic
- `wg.Add(1)` before `go func()` — not inside the goroutine (race condition otherwise)
- `event, ok := <-p.jobs` — `ok` check is mandatory, closed channel returns zero values forever
- `close` before `Wait` — signal workers first, then wait for them to finish
- Mutex protects shared counter in concurrent tests — always run with `-race`

**Key Go concepts:**
- `make(chan T, n)` — buffered channel, `workerCount*2` is a common convention
- `p.wg.Add(1)` before goroutine launch — not inside it
- `defer p.wg.Done()` — first line of every worker goroutine
- `event, ok := <-ch` — ok is false when channel is closed
- `close(ch)` — signals consumers, unblocks workers blocked on select
- `sync.Mutex` — protects shared state across goroutines in tests
- `go test -race` — detects data races at runtime

**What confused me at first:**
- Why `close` must come before `wg.Wait()` in `Stop()`.
  Workers are blocked on `select` waiting for a job or ctx.Done().
  Without `close`, nothing unblocks them. `wg.Wait()` deadlocks.

**What finally made it clear:**
- Signal first, wait second. `close` is the signal. `wg.Wait()` is the wait.
  The same rule applies everywhere in Go concurrency.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| `wg.Add(1)` inside goroutine | Race — `Wait()` may return before goroutines start |
| Missing `ok` check on channel receive | Closed channel returns zero values forever — infinite loop |
| `wg.Wait()` before `close(jobs)` | Deadlock — workers blocked on select, Wait never returns |
| No mutex on shared counter in tests | Data race — incorrect count, undefined behaviour |
| Hardcoding business logic in the pool | Pool becomes non-reusable — keep it generic |

---

## 3. Code Demo

### `internal/worker/pool.go`

```go
package worker

import (
    "context"
    "sync"

    "github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type WorkerPool struct {
    jobs        chan domain.NotificationEvent
    workerCount int
    wg          sync.WaitGroup
}

func NewWorkerPool(workerCount int) *WorkerPool {
    return &WorkerPool{
        jobs:        make(chan domain.NotificationEvent, workerCount*2),
        workerCount: workerCount,
    }
}

func (p *WorkerPool) Start(ctx context.Context, process func(ctx context.Context, event domain.NotificationEvent)) {
    for i := 0; i < p.workerCount; i++ {
        p.wg.Add(1)
        go func() {
            defer p.wg.Done()
            for {
                select {
                case event, ok := <-p.jobs:
                    if !ok {
                        return
                    }
                    process(ctx, event)
                case <-ctx.Done():
                    return
                }
            }
        }()
    }
}

func (p *WorkerPool) Stop() {
    close(p.jobs)
    p.wg.Wait()
}

func (p *WorkerPool) Submit(event domain.NotificationEvent) {
    p.jobs <- event
}
```

### `internal/worker/pool_test.go`

```go
package worker

import (
    "context"
    "sync"
    "testing"

    "github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

func TestWorkerPoolProcessesEvents(t *testing.T) {
    tests := []struct {
        name        string
        workerCount int
        events      []domain.NotificationEvent
        wantCount   int
    }{
        {
            name:        "processes all submitted events",
            workerCount: 3,
            events: []domain.NotificationEvent{
                {ID: "evt-001"},
                {ID: "evt-002"},
                {ID: "evt-003"},
                {ID: "evt-004"},
                {ID: "evt-005"},
            },
            wantCount: 5,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            pool := NewWorkerPool(tt.workerCount)
            ctx := context.Background()

            var mu sync.Mutex
            processedCount := 0

            pool.Start(ctx, func(ctx context.Context, event domain.NotificationEvent) {
                mu.Lock()
                processedCount++
                mu.Unlock()
            })

            for _, event := range tt.events {
                pool.Submit(event)
            }

            pool.Stop()

            if processedCount != tt.wantCount {
                t.Errorf("got processed count %d, want %d", processedCount, tt.wantCount)
            }
        })
    }
}
```

### Running with race detector

```bash
go test -race ./internal/worker/...
# PASS — no race conditions
```

---

## 4. Interview Takeaway

**What a worker pool is:**
A fixed number of goroutines that read jobs from a buffered channel and process them concurrently.
It decouples job submission from job processing and limits concurrency to a controlled number.

**How it differs from Java's ThreadPoolExecutor:**
Same concept — different implementation. Go uses goroutines and channels directly.
No framework component, no configuration XML, no annotations.
The pool is a plain struct with Start, Stop, and Submit methods.

**The `process` function parameter pattern:**
The pool is generic — it does not know what processing means.
A function is injected at startup. This follows Go's preference for small, composable pieces
over large framework abstractions.

**Why `close` before `wg.Wait()`:**
Workers block on `select` waiting for a job or cancellation.
`close(channel)` is the signal that unblocks them — they see `ok == false` and exit.
Without `close`, workers never unblock and `wg.Wait()` deadlocks.
Rule: signal first, wait second.

**How to test concurrent code:**
Use a mutex-protected counter to safely count operations across goroutines.
Always run with `go test -race` to detect data races the compiler cannot catch.

---

## 5. Cleanup Notes

Day 15 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/worker/pool.go` — the worker pool, used by the dispatcher in Day 17
- `internal/worker/pool_test.go` — concurrent tests, always run with `-race`

**Note:** The pool is not connected to the dispatcher or notifiers yet.
That happens on Day 17 when the full dispatch flow is wired together.