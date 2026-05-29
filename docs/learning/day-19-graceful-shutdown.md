# Day 19 — Graceful Shutdown

---

## 1. Original Lesson Explanation

### 1.1 Big picture

Pressing Ctrl+C without graceful shutdown causes three problems:

- A worker mid-job is killed — the event stays in `PROCESSING` forever, never retried
- The database connection pool is dropped without cleanup — potential connection leaks on the DB side
- The HTTP server stops accepting requests mid-flight — clients get connection reset errors

Graceful shutdown means: **stop accepting new work, finish current work, then exit cleanly.**

---

### 1.2 The shutdown sequence

```
SIGTERM or SIGINT received
        ↓
1. Stop HTTP server — no new requests accepted
2. Cancel context — stops dispatcher and workers from accepting new work
3. Stop worker pool — wait for in-flight workers to finish
4. Close database pool — clean disconnect
        ↓
Process exits cleanly
```

Each step must complete before the next begins. Sequential, not concurrent.

---

### 1.3 Signal handling in Go

```go
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit // blocks until signal received
```

`signal.Notify` registers the channel to receive the specified signals.
`<-quit` blocks until a signal arrives.
Buffer of 1 is important — if the program is busy when the signal arrives, it is not lost.

> **Türkçe özet:** OS'tan SIGINT (Ctrl+C) ya da SIGTERM sinyali gelince `quit` channel'ına
> değer gelir. `<-quit` bu değeri bekler. Sinyal gelince shutdown sequence başlar.

---

### 1.4 Context cancellation as the shutdown trigger

Replace `context.Background()` with a cancellable context:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()
```

When `cancel()` is called:
- The dispatcher's ticker loop sees `ctx.Done()` and exits
- Worker goroutines see `ctx.Done()` and exit after finishing their current job
- In-progress DB queries respect the cancelled context

This is why context flows through every layer — it is the shutdown mechanism.

---

### 1.5 Stopping the HTTP server gracefully

`http.ListenAndServe` does not support graceful shutdown.
Use `http.Server` with `Shutdown`:

```go
srv := &http.Server{
    Addr:    ":8080",
    Handler: r,
}

go func() {
    log.Println("server listening on :8080")
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatal("server error:", err)
    }
}()
```

`http.ErrServerClosed` is returned by `ListenAndServe` after `Shutdown` is called.
This is normal — not an error. Without this check, `log.Fatal` fires on clean shutdown.

```go
shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
defer shutdownCancel()

if err := srv.Shutdown(shutdownCtx); err != nil {
    log.Printf("server shutdown error: %v", err)
}
```

`srv.Shutdown` stops accepting new requests and waits for in-flight requests to complete.
The 10 second timeout prevents waiting forever if a request is stuck.

---

### 1.6 The buffered channel race — a known limitation

**The problem:**
When `cancel()` is called, workers select on both `ctx.Done()` and the jobs channel.
If a job was submitted just before cancellation and both cases are ready simultaneously,
Go picks randomly. A worker may choose `ctx.Done()` and exit — leaving the job unprocessed.

**Your current implementation:**
```go
select {
case event, ok := <-p.jobs:
    if !ok { return }
    process(ctx, event)
case <-ctx.Done():
    return  // may exit with jobs still in the buffer
}
```

**The production-grade solution — drain to empty:**
```go
go func() {
    defer p.wg.Done()
    for event := range p.jobs {  // range exits only when closed AND empty
        process(ctx, event)
    }
}()
```

`range` on a channel reads until the channel is both closed and empty.
Workers no longer watch `ctx.Done()` — `close(p.jobs)` is the shutdown signal.
The channel is fully drained before workers exit.

**Tradeoff:** workers no longer respond to context cancellation mid-processing.
Shutdown waits for all queued jobs to finish. For fast notification dispatch, acceptable.
For long-running jobs, the current select-based approach is more responsive.

**For the MVP:** current implementation is fine. The window where a job can be lost is tiny.
Document as a known limitation in the README.

---

## 2. My Learning Summary

**What I learned:**
- Graceful shutdown requires four steps in order: HTTP server → cancel context → worker pool → DB pool
- `signal.Notify` with a buffered channel of 1 — signal is never lost
- `context.WithCancel` replaces `context.Background()` — `cancel()` is the shutdown trigger
- `http.Server.Shutdown` drains in-flight requests with a timeout
- `http.ErrServerClosed` is normal after `Shutdown` — not an error
- Workers with `select` may miss buffered jobs on shutdown — drain-to-empty fixes this

**Key Go concepts:**
- `os/signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)` — intercept OS signals
- `context.WithCancel(parent)` — returns ctx and cancel function
- `defer cancel()` — safety net, always cancels even on unexpected exit
- `http.Server{Addr, Handler}` + `srv.Shutdown(ctx)` — graceful HTTP shutdown
- `http.ErrServerClosed` — expected error after Shutdown, not fatal
- `range ch` on a channel — drains fully before exiting, unlike select

**What confused me at first:**
- Is a buffered job guaranteed to be processed after `cancel()`?
  No. Workers select on both `ctx.Done()` and the jobs channel.
  Go picks randomly when both are ready — the job may be skipped.

**What finally made it clear:**
- `range` on a channel reads until closed AND empty — no random selection.
  Replacing `select` with `range` in workers guarantees full drain on shutdown.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Unbuffered signal channel | Signal lost if program is busy when it arrives |
| Not checking `http.ErrServerClosed` | `log.Fatal` fires on clean shutdown — false error |
| Closing DB pool before worker pool stops | Workers may still be writing to DB when connection closes |
| No timeout on `srv.Shutdown` | Waits forever if a request is stuck |
| `defer cancel()` missing | Context never cancelled on unexpected exit — goroutine leaks |

---

## 3. Code Demo

### `cmd/server/main.go` — full graceful shutdown

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// ... setup pool, repository, service, handler, dispatcher, workerPool ...

srv := &http.Server{
    Addr:    ":8080",
    Handler: r,
}

go func() {
    log.Println("server listening on :8080")
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatal("server error:", err)
    }
}()

// wait for OS signal
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

log.Println("shutdown signal received")

// 1. cancel context — stops dispatcher and workers from new work
cancel()

// 2. stop HTTP server — drain in-flight requests
shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
defer shutdownCancel()
if err := srv.Shutdown(shutdownCtx); err != nil {
    log.Printf("server shutdown error: %v", err)
}

// 3. stop worker pool — wait for in-flight jobs
workerPool.Stop()

// 4. close DB pool — clean disconnect
pool.Close()

log.Println("shutdown complete")
```

### drain-to-empty worker pattern (production-grade alternative)

```go
// in pool.go — replace select with range
go func() {
    defer p.wg.Done()
    for event := range p.jobs {
        process(ctx, event)
    }
}()

// Stop — close channel, wait for drain
func (p *WorkerPool) Stop() {
    close(p.jobs)
    p.wg.Wait()
}
```

---

## 4. Interview Takeaway

**What graceful shutdown means:**
Stop accepting new work, finish current work, then exit cleanly.
Four steps in order: HTTP server → cancel context → worker pool → DB pool.
Each step waits for the previous to complete.

**How Go handles OS signals:**
`os/signal.Notify` registers a buffered channel to receive `SIGINT` and `SIGTERM`.
The main goroutine blocks on `<-quit`. When the signal arrives, the shutdown sequence begins.

**The buffered channel race on shutdown:**
Workers using `select` on both `ctx.Done()` and the jobs channel may skip buffered jobs
when context is cancelled — Go picks randomly between ready cases.
The production fix: use `range` on the channel instead of `select`.
`range` reads until the channel is closed AND empty — guaranteed full drain.

**`http.ErrServerClosed`:**
`srv.ListenAndServe` returns this error after `srv.Shutdown` is called.
It is expected and normal — not a fatal error. Always check for it explicitly.

---

## 5. Cleanup Notes

Day 19 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `cmd/server/main.go` — updated with graceful shutdown, signal handling, `http.Server`

**Known limitation documented:**
The current worker pool `select` pattern may skip buffered jobs on shutdown.
The drain-to-empty `range` pattern is the production-grade fix.
This is documented as a known limitation — acceptable for the MVP.