# Day 28 — Interview Prep: Go Concepts

A personal cheat sheet of the 10 most common Go backend interview questions,
answered with concrete references to the notification dispatcher project.

**Principle:** state the concept, then immediately give a project example.
Two to four sentences each — spoken interview length, not essays.

---

## Q1 — Goroutine vs OS thread

Goroutines are managed by the Go runtime, not the OS — around 2KB each, so you can
run hundreds of thousands of them. An OS thread is around 1MB with slow OS-level
context switching. The Go runtime multiplexes many goroutines onto a small number
of OS threads, so I get cheap concurrency without managing thread pools.

In my notification service the worker pool runs goroutines, and I can scale the
worker count freely without worrying about thread exhaustion.

---

## Q2 — Channels: buffered vs unbuffered

A channel is a typed pipe for communication between goroutines.
Unbuffered — the sender blocks until a receiver takes the value; they synchronise
at the handoff. Buffered — the sender only blocks when the buffer is full, so producer
and consumer are decoupled in time.

In my service I used a **buffered** channel for the worker pool (`workerCount*2`),
because the whole point is to decouple job submission from processing — the dispatcher
submits without waiting for a free worker.

---

## Q3 — What is context.Context and why does it matter

`context.Context` propagates cancellation, deadlines, and timeouts through a call chain.
Every operation that can take time — a DB query, HTTP call, notification dispatch —
receives a context. If the context is cancelled or times out, the operation stops.

In my service, context flows from the HTTP handler through the service into the database
calls. I also use it for graceful shutdown — cancelling the root context signals the
dispatcher and workers to stop. My timeout middleware wraps each request with a deadline
using the request's context.

---

## Q4 — How Go handles errors vs exceptions

Go has no exceptions. Errors are just values — a function returns an error as its last
return value and the caller checks it immediately with `if err != nil`. You wrap errors
with `fmt.Errorf` and `%w` to add context while preserving the original, and check them
with `errors.Is` and `errors.As`.

In my service I defined a custom `NotifyError` type with an event ID and reason.
The tradeoff versus exceptions is more verbose code, but the error flow is completely
explicit — you always see where an error can occur and where it is handled.

---

## Q5 — How interfaces work in Go vs Java

In Java a class declares `implements`. In Go a type satisfies an interface just by having
the right methods — it does not need to know the interface exists. This means I can define
an interface in one package and the implementation in another, which keeps things loosely
coupled. Go also favours small interfaces, defined where they are used.

In my service, the `Notifier` interface lives in the domain package because that is where
it is consumed. `FakeEmailNotifier` and `FakeWebhookNotifier` implement it in a separate
package — so the dependency only flows one way.

---

## Q6 — How to prevent race conditions in Go

Three ways. First, channels — the idiomatic Go approach. Instead of sharing memory and
locking it, you pass data through channels so only one goroutine owns it at a time.
That is what my worker pool does. Second, when you must share state, use a mutex —
`RWMutex` for read-heavy cases. Third, `go test -race` to detect races the compiler misses.

I run all my tests with the race flag. My worker pool test uses a mutex-protected counter
precisely so it stays race-free under `-race`. My notifier registry is built once at
startup and only read after, so it is safe without locks.

---

## Q7 — How to implement graceful shutdown

I use a buffered signal channel that receives SIGINT and SIGTERM. When a signal arrives,
I run the shutdown sequence in order: first cancel the root context, which signals the
dispatcher and workers to stop taking new work. Then shut down the HTTP server to drain
in-flight requests. Then stop the worker pool, which waits for in-flight jobs. Finally
close the database pool.

I also learned to use a separate context for in-flight job processing, so a job already
running finishes cleanly instead of being cancelled mid-way and left half-processed.

---

## Q8 — How to structure a Go backend project

`cmd/` is where execution starts — one folder per binary. `internal/` is private code the
Go compiler prevents other modules from importing. `pkg/` is optional, for code meant to be
reused externally — I did not need it. `go.mod` defines the module and Go version.

Inside `internal/` I split by responsibility: handler, service, repository, domain, worker,
dispatcher — each layer depends on interfaces, not concrete types.

---

## Q9 — How to test Go code without a real database

I test without a real database by using interfaces and fake implementations. My service
depends on an `EventRepository` interface, not the PostgreSQL implementation. In tests I
create a fake repository that records what was called and returns whatever the test needs —
no database, no network. My handler tests use a fake service the same way.

This makes tests fast and deterministic — they run in milliseconds. For testing the actual
SQL, I would write separate integration tests against a real database in Docker, but those
are slower and run separately.

---

## Q10 — Why choose Go over Java for a backend service

A few concrete reasons from building this service. First, deployment — my Go Docker image
is 34MB; the equivalent Spring Boot image would be 200 to 400MB. For a service deployed
frequently in Kubernetes, that matters. Second, concurrency — goroutines and channels are
simpler and cheaper than thread pools for async dispatch work. Third, startup time and
memory — no JVM warm-up, low memory footprint.

That said, Java still has a richer ecosystem and is better for large enterprise systems with
heavy framework needs. Go fits best for lightweight, high-concurrency network services.

---

## Interview delivery reminders

- **State the concept, then the project example.** Two sentences minimum per answer.
- **A too-short answer reads as shallow knowledge** — even when it is not.
- **Honesty beats memorisation.** "I used X in my project, still learning the edge cases"
  beats a memorised answer you cannot defend.
- **Acknowledge tradeoffs.** Mentioning Java's strengths in Q10 makes you sound balanced.
- **Every answer can point to real code you wrote.** That is the advantage over candidates
  who only read about Go.

---

## Common slip to avoid

In an early draft I said I used an "unbuffered" channel for the worker pool.
It is actually **buffered** (`make(chan ..., workerCount*2)`).
Know your own code precisely — interviewers may check.