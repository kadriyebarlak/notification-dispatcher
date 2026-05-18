# Day 05 — Goroutines & Channels

---

## 1. Original Lesson Explanation

### 1.1 Big picture

In Java, concurrency means threads. Threads are expensive — each one takes around 1MB of memory
and OS-level context switching is slow. You manage thread pools carefully because threads are a limited resource.

In Go, concurrency means goroutines. A goroutine starts at around 2KB of memory and is managed
by the Go runtime, not the OS. You can run thousands of them without thinking twice.

But goroutines are only useful if they can communicate. That is where **channels** come in.

---

### 1.2 Why it exists

Go was designed for networked, concurrent systems from the beginning.
The language needed a concurrency model that was simple enough to reason about,
but powerful enough for real production workloads.

Go's answer is summarised in one famous line:

> *Do not communicate by sharing memory. Share memory by communicating.*

In Java you share state between threads using locks, synchronized blocks, and volatile variables.
In Go you pass data between goroutines through channels.
The channel owns the data. Only one goroutine touches it at a time.

> **Türkçe özet:** Java'da thread'ler arasında veri paylaşmak için lock kullanırsın.
> Go'da ise goroutine'ler arasında channel üzerinden veri gönderirsin.
> Veriyi paylaşmak yerine, veriyi taşırsın. Bu daha az hata üretir çünkü
> aynı anda iki goroutine aynı veriye erişemez.

---

### 1.3 Goroutines

Starting a goroutine is one keyword:

```go
go doSomething()
```

No thread pool configuration, no `ExecutorService`, no `@Async`.

**Important:** if `main()` exits, all goroutines are killed immediately — even if they are mid-work.
You need to wait for them. That is what `sync.WaitGroup` is for:

```go
var wg sync.WaitGroup

wg.Add(1)
go func() {
    defer wg.Done()
    doSomething()
}()

wg.Wait() // blocks until Done() is called
```

---

### 1.4 Channels

A channel is a typed pipe. You send values in one end and receive from the other.

```go
ch := make(chan string)     // unbuffered
ch := make(chan string, 10) // buffered — holds up to 10 values
```

**Sending and receiving:**
```go
ch <- "hello"   // send
msg := <-ch     // receive
```

**Unbuffered channel** — the sender blocks until the receiver takes the value.
They must meet at the channel. The synchronisation point is the transfer of the value — not the completion of work.
Think of it like handing someone a package: the moment they take it from your hands, you are free to go.
You do not wait for them to open it.

**Buffered channel** — the sender only blocks when the buffer is full.
The receiver only blocks when the buffer is empty.
Producer and consumer are decoupled in time.

---

### 1.5 Buffered vs unbuffered — which to use for a worker pool

| | Unbuffered | Buffered |
|---|---|---|
| Producer blocks until | A worker takes the job | Buffer is full |
| Consumer blocks until | A job is sent | Buffer is empty |
| Producer and consumer | Must meet (synchronous handoff) | Decoupled in time |
| Good for worker pool | No — HTTP handler would wait for a free worker | Yes — job submission and processing are independent |

For a worker pool, always use a **buffered channel**.
The whole point is to decouple job submission from job processing.

---

### 1.6 The producer/consumer pattern

This is the exact shape the worker pool uses in week 3:

```go
jobs := make(chan string, 5)

// producer
go func() {
    for _, j := range []string{"job1", "job2", "job3"} {
        jobs <- j
    }
    close(jobs) // signal: no more jobs coming
}()

// consumer
for job := range jobs { // stops automatically when channel is closed
    fmt.Println("processing:", job)
}
```

`close(jobs)` tells consumers there is nothing more coming.
`range` on a channel automatically stops when the channel is closed.

---

### 1.7 Stopping goroutines with context

A goroutine cannot be killed from outside. You can only ask it to stop —
through a channel signal or a context cancellation.

```go
func worker(ctx context.Context, jobs <-chan string) {
    for {
        select {
        case job, ok := <-jobs:
            if !ok {
                return // channel closed
            }
            fmt.Println("processing:", job)
        case <-ctx.Done():
            return // context cancelled
        }
    }
}
```

This `select` pattern — jobs channel vs context cancellation — is the heart of the worker pool.

> **Türkçe özet:** Goroutine'i dışarıdan öldüremezsin. Ona "dur" diyebilirsin —
> context iptal edilince ya da channel kapanınca. Worker pool'unda bu iki sinyal yarışır:
> ya yeni iş gelir, ya da context iptal edilir.

---

### 1.8 Channel direction types

Go lets you restrict what a function can do with a channel:

```go
chan<- string   // send-only — producer
<-chan string   // receive-only — consumer
chan string     // bidirectional — used when creating the channel
```

Use directional types in function signatures to express intent clearly
and prevent accidental misuse.

---

## 2. My Learning Summary

**What I learned:**
- Goroutines are lightweight — ~2KB, managed by the Go runtime, not the OS
- Channels are typed pipes for communication between goroutines
- Unbuffered channels synchronise at the moment of value transfer — not when work is done
- Buffered channels decouple producer and consumer in time
- A goroutine cannot be killed — it must be asked to stop via context or channel close
- `wg.Add(1)` must be called before `go func()` — not inside the goroutine

**Key Go concepts:**
- `go func()` — starts a goroutine
- `make(chan T, n)` — creates a buffered channel of capacity n
- `close(ch)` — signals no more values; consumers exit their `range` loop
- `job, ok := <-jobs` — `ok` is `false` when channel is closed; always check it
- `sync.WaitGroup` — wait for a group of goroutines to finish
- `chan<-` / `<-chan` — directional channel types for function parameters
- `select` — blocks until one case is ready; picks randomly if multiple are ready

**What confused me at first:**
- With an unbuffered channel, does the producer wait for the worker to finish processing?
  No — the producer unblocks the moment the worker **takes** the job from the channel.
  What happens after the handoff is none of the producer's concern.

**What finally made it clear:**
- The package handoff analogy: the moment someone takes the package from your hands, you are free.
  You do not wait for them to open it.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| `wg.Add(1)` inside the goroutine | Race condition — `Wait()` may return before goroutines start |
| Not checking `ok` on channel receive | Closed channel returns zero values forever — infinite loop |
| Forgetting `close(jobs)` in producer | Consumers block forever waiting for more jobs |
| Using `context.Background()` instead of incoming `ctx` for child context | Breaks cancellation chain |
| Using unbuffered channel for worker pool job queue | HTTP handler blocks until a worker is free |

---

## 3. Code Demo

### `produce` — sends jobs, respects context, closes channel on exit

```go
func produce(ctx context.Context, jobs chan<- string) {
    defer close(jobs)

    eventIDs := []string{"evt-001", "evt-002", "evt-003", "evt-004", "evt-005"}

    for _, eventID := range eventIDs {
        select {
        case jobs <- eventID:
            fmt.Println("produced:", eventID)
        case <-ctx.Done():
            fmt.Println("producer stopped:", ctx.Err())
            return
        }
    }
}
```

### `consume` — reads jobs, respects context, signals WaitGroup on exit

```go
func consume(ctx context.Context, workerID int, jobs <-chan string, wg *sync.WaitGroup) {
    defer wg.Done()

    for {
        select {
        case job, ok := <-jobs:
            if !ok {
                fmt.Println("worker stopped, jobs channel closed:", workerID)
                return
            }
            fmt.Println("worker", workerID, "processing:", job)
            time.Sleep(500 * time.Millisecond)
        case <-ctx.Done():
            fmt.Println("worker stopped by context:", workerID, ctx.Err())
            return
        }
    }
}
```

### `runDemo` — wires producer and 3 consumers

```go
func runDemo() {
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
    defer cancel()

    jobs := make(chan string, 5)

    go produce(ctx, jobs)

    var wg sync.WaitGroup
    for workerID := 1; workerID <= 3; workerID++ {
        wg.Add(1)
        go consume(ctx, workerID, jobs, &wg)
    }

    wg.Wait()
    fmt.Println("concurrency demo finished")
}
```

### Why scheduling is non-deterministic

Two sources of non-determinism:
1. The Go scheduler decides which goroutine runs when — unpredictable
2. When multiple `select` cases are ready simultaneously, Go picks one **at random**

You cannot predict or control which worker processes which job.

---

## 4. Interview Takeaway

**Goroutines vs threads:**
Goroutines are managed by the Go runtime, not the OS. They start at ~2KB vs ~1MB for OS threads.
You can run hundreds of thousands of goroutines; the same is not true for threads.
The Go scheduler multiplexes goroutines onto a small number of OS threads automatically.

**How goroutines communicate:**
Through channels — typed pipes that transfer ownership of data.
This avoids shared memory and eliminates most locking bugs.
Go's principle: do not communicate by sharing memory, share memory by communicating.
in turkish: Ortak veriyi paylaşarak konuşma; mesaj göndererek veriyi aktar.

**Buffered vs unbuffered channels:**
Unbuffered: sender and receiver must meet — synchronous handoff at the point of transfer.
Buffered: sender and receiver are decoupled — sender only blocks when buffer is full.
For a worker pool job queue, always use buffered.

**How to stop a goroutine:**
You cannot kill a goroutine from outside. Signal it to stop via context cancellation
(`ctx.Done()`) or by closing a channel. The goroutine decides when to exit.

**`sync.WaitGroup` pattern:**
Call `wg.Add(1)` before launching each goroutine — never inside the goroutine.
Use `defer wg.Done()` as the first line of the goroutine body.
Call `wg.Wait()` to block until all goroutines finish.

---

## 5. Cleanup Notes

The following code was written as a **temporary learning exercise** and must not be committed to the main application code.

**Delete:**
- `cmd/server/concurrency_demo.go` — delete the entire file

**Clean up:**
- `cmd/server/main.go` — remove the `runDemo()` call.
  `main.go` should return to only the interface checks and the startup message.

**Keep:**
- `docs/learning/day-05-goroutines-channels.md` — this file