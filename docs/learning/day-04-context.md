# Day 04 — context.Context

---

## 1. Original Lesson Explanation

### 1.1 Big picture

In Spring Boot, when an HTTP request comes in, the framework manages its lifecycle for you.
Timeouts, cancellation, request-scoped values — Spring handles it invisibly in the background.

In Go, **you manage this yourself, explicitly, through `context.Context`.**
Every operation that can take time — an HTTP call, a database query, a notification dispatch — receives a context.
If the context is cancelled or times out, the operation should stop.

This is Go's answer to: *"What happens when the caller gives up but the work is still running?"*

---

### 1.2 Why it exists

Imagine a user sends an HTTP request. Your handler calls a service, which calls a database, which calls an external API.
The user disconnects after 2 seconds. In Java with Spring, the database query and external API call keep running — wasting resources.

In Go, the HTTP request comes with a context. When the user disconnects, **that context is automatically cancelled.**
If every function in the chain respects the context, they all stop immediately. No wasted work.

> **Türkçe özet:** Kullanıcı bağlantıyı kestiğinde ya da timeout dolduğunda, context iptal edilir.
> Her fonksiyon bu context'i dinliyorsa, hepsi durur.
> Kullanıcı bağlantıyı kestiğinde Go’daki HTTP request context’i otomatik iptal edilir. Eğer handler, service, DB call ve external API call bu context’i kullanıyor ve dinliyorsa, işlem zinciri durabilir.
> Spring’de request lifecycle framework tarafından yönetilir, ama çalışan DB query veya external API call’ın otomatik duracağı garanti değildir. Go’da cancellation daha explicit görünür: context’i zincir boyunca sen geçirirsin ve fonksiyonlar buna saygı duymalıdır.

---

### 1.3 The three things context does

**1 — Cancellation**
```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel() // always call cancel to free resources
```
When you call `cancel()`, the context is cancelled.
Any function watching `ctx.Done()` will know to stop.

---

**2 — Timeout**
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```
Automatically cancels after 5 seconds.
After that, `ctx.Err()` returns `context.DeadlineExceeded`.

---

**3 — Deadline**
```go
ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
defer cancel()
```
Same as timeout but you give an absolute time instead of a duration.
In practice, `WithTimeout` is more common.

---

### 1.4 How a function respects context

A function that does slow work should watch `ctx.Done()`:

```go
func doSlowWork(ctx context.Context) error {
    select {
    case <-time.After(3 * time.Second):
        return nil // work finished
    case <-ctx.Done():
        return ctx.Err() // cancelled or timed out
    }
}
```

`select` blocks until one of the cases is ready.
Whichever channel receives first wins.
This is the core pattern used in the worker pool in week 3.

---

### 1.5 Two rules you must know

**Rule 1 — Context is always the first parameter:**
```go
func (e EmailNotifier) Send(ctx context.Context, event domain.NotificationEvent) error
```
Not in a struct. Not optional. Always first.
This is a Go community convention so strong it is essentially a rule.

**Rule 2 — Never store context in a struct:**
```go
// wrong
type EmailNotifier struct {
    ctx context.Context
}

// correct — pass it through function parameters
func (e EmailNotifier) Send(ctx context.Context, ...) error
```
Context belongs to a single call chain, not to a long-lived object.

> **Türkçe özet:** Context her zaman fonksiyonun ilk parametresidir.
> Struct içine koyma — çünkü context bir isteğin ya da işlemin yaşam süresine aittir, struct'ın değil.

---

## 2. My Learning Summary

**What I learned:**
- `context.Context` is Go's explicit mechanism for managing the lifecycle of a request or operation
- Every slow function should accept `ctx` as its first parameter and respect `ctx.Done()`
- Child contexts inherit cancellation from their parent automatically
- `defer cancel()` must always follow `WithTimeout` or `WithCancel`

**Key Go concepts:**
- `context.WithTimeout` — creates a child context with a duration-based deadline
- `context.WithCancel` — creates a child context you cancel manually
- `ctx.Done()` — a channel closed when the context is cancelled or expired
- `ctx.Err()` — returns `context.Canceled` or `context.DeadlineExceeded`
- `select` statement — blocks until one channel is ready, used to race work against context cancellation

**What confused me at first:**
- The difference between `context.Canceled` and `context.DeadlineExceeded` — which error comes back depends on what fired first: the parent being cancelled, or the child's own timeout

**What finally made it clear:**
- Context forms a tree. Cancel a parent and every child is cancelled immediately.
- Writing the `select` with `ctx.Done()` and seeing the timeout fire before the fake work finished made the concept real.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Using `context.Background()` inside a child function instead of the incoming `ctx` | Breaks the cancellation chain silently |
| Forgetting `defer cancel()` | Resource leak until timeout fires |
| Storing `ctx` in a struct field | Context belongs to a call, not an object |
| Ignoring `ctx.Err()` after `ctx.Done()` fires | You lose the reason: cancelled vs timed out |

---

## 3. Code Demo

### `dispatchWithTimeout` — creates a child context and calls slow work

```go
func dispatchWithTimeout(ctx context.Context, eventID string) error {
    // create a child context from the incoming parent — not from Background()
    ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()

    err := fakeNotify(ctx, eventID)
    if err != nil {
        return fmt.Errorf("failed to dispatch event %s: %w", eventID, err)
    }
    return nil
}
```

### `fakeNotify` — simulates slow work and respects context cancellation

```go
func fakeNotify(ctx context.Context, eventID string) error {
    select {
    case <-time.After(4 * time.Second): // slow work — takes 4 seconds
        fmt.Println("notification sent for event:", eventID)
        return nil
    case <-ctx.Done(): // context cancelled or timed out
        return ctx.Err()
    }
}
```

### Calling from `main()`

```go
err := dispatchWithTimeout(context.Background(), "evt-001")
if err != nil {
    fmt.Println("dispatch with timeout error:", err)
    return
}
fmt.Println("dispatch with timeout succeeded")
```

### Timeout vs success behavior

| Timeout setting | Fake work duration | Result |
|---|---|---|
| `2 * time.Second` | 4 seconds | `context.DeadlineExceeded` — timeout fires first |
| `5 * time.Second` | 4 seconds | success — work finishes before timeout |

---

## 4. Interview Takeaway

**What `context.Context` is:**
Go's mechanism for propagating cancellation, deadlines, and timeouts through a call chain.
It replaces what Spring Boot manages invisibly in the request lifecycle.

**Why it is passed as the first parameter:**
Community convention — strong enough to be treated as a rule.
It signals that the function respects the caller's lifecycle and can be stopped.

**How cancellation flows from parent to child:**
Context forms a tree. When a parent context is cancelled — whether by timeout, deadline, or manual cancel — every child context derived from it is cancelled immediately.
You do not need to do anything extra. Go handles the propagation.

**Difference between `context.Canceled` and `context.DeadlineExceeded`:**
- `context.Canceled` — someone called `cancel()` explicitly, or the parent was cancelled
- `context.DeadlineExceeded` — the timeout or deadline expired before the work finished

Check them separately in production when the reason matters:
```go
if errors.Is(err, context.DeadlineExceeded) {
    // log a timeout warning
}
if errors.Is(err, context.Canceled) {
    // caller gave up — not necessarily an error
}
```

---

## 5. Cleanup Notes

The following code was written as a **temporary learning exercise** and must not be committed to the main application code.

**Delete:**
- `cmd/server/context_demo.go` — delete the entire file

**Clean up:**
- `cmd/server/main.go` — remove the `dispatchWithTimeout` call and the `"context demo starting..."` print line.
  `main.go` should return to only the interface checks and the startup message.