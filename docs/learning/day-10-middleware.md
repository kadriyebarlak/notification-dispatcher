# Day 10 — Middleware: Logging & Request Timeout

---

## 1. Original Lesson Explanation

### 1.1 Big picture

After Day 9, the server handles requests but logs nothing and has no timeout protection.
A slow client or a hanging database call could hold a goroutine forever.

Middleware solves both problems. In Go, middleware is just **a function that wraps a handler**.
No annotations, no AOP, no framework magic — just functions calling functions.

---

### 1.2 Why it exists

In Spring Boot you use filters, interceptors, or AOP for cross-cutting concerns like logging,
auth, and timeouts. The framework wires them for you.

In Go, middleware is simpler. A middleware is a function that:
- receives an `http.Handler`
- returns a new `http.Handler`
- does something before and/or after calling the original handler

```go
func MyMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // do something before
        next.ServeHTTP(w, r)
        // do something after
    })
}
```

That pattern is all middleware is in Go. One function wrapping another.

> **Türkçe özet:** Spring'de middleware için filter veya interceptor yazarsın, framework bağlar.
> Go'da middleware sadece bir handler'ı alıp yeni bir handler döndüren fonksiyondur.
> "Before" ve "after" mantığı açıkça görünür — gizli hiçbir şey yok.

---

### 1.3 Logging middleware

You want to log every request: method, path, status code, and duration.

The tricky part is **status code**. `http.ResponseWriter` does not expose the status code
after it is written. You need to wrap it using **embedding**:

```go
type responseWriter struct {
    http.ResponseWriter
    status int
}

func (rw *responseWriter) WriteHeader(status int) {
    rw.status = status
    rw.ResponseWriter.WriteHeader(status)
}
```

`responseWriter` embeds `http.ResponseWriter` — it gets all its methods automatically,
and you only override `WriteHeader` to capture the status code.
The default status is set to `http.StatusOK` so handlers that never call `WriteHeader`
explicitly still log the correct status.

```go
func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
        next.ServeHTTP(rw, r)
        log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
    })
}
```

---

### 1.4 Timeout middleware

Every request should have a maximum time limit. This is `context.WithTimeout` from Day 4
applied directly to the HTTP request:

```go
func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx, cancel := context.WithTimeout(r.Context(), timeout)
            defer cancel()
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

Two important details:
- `r.Context()` — the HTTP request already carries a context. Create a child from it,
  not from `context.Background()`. Same rule as Day 4.
- `r.WithContext(ctx)` — returns a new request with the updated context.
  The original request is not modified.

---

### 1.5 Middleware ordering

Middleware wraps from outside in. With this wiring:

```go
r.Use(LoggingMiddleware)
r.Use(TimeoutMiddleware(5 * time.Second))
```

The request flows:
```
LoggingMiddleware → TimeoutMiddleware → Handler
```

The response flows back:
```
Handler → TimeoutMiddleware → LoggingMiddleware
```

**Logging goes outermost** — it wraps everything and sees:
- The final status code including any written by inner middleware
- The total duration including timeout overhead

If timeout were outermost and it fired, writing a 503 response,
logging would never see that status code.

**General rule: observability middleware goes outermost.**
Logging, tracing, and metrics always go first so they see the complete picture.

---

### 1.6 Wiring middleware with chi

```go
r := chi.NewRouter()
r.Use(handler.LoggingMiddleware)
r.Use(handler.TimeoutMiddleware(5 * time.Second))
r.Post("/events", eventHandler.CreateEvent)
```

`r.Use()` applies middleware to every route on that router. Order matters — top to bottom.

---

## 2. My Learning Summary

**What I learned:**
- Middleware is just a function that takes a handler and returns a handler — no magic
- Struct embedding lets you override one method and inherit all others automatically
- The default status in `responseWriter` must be `http.StatusOK` — otherwise handlers
  that skip `WriteHeader` log status `0`
- `r.Context()` not `context.Background()` — always create child contexts from the request
- `r.WithContext(ctx)` — creates a new request with updated context, original unchanged
- Logging goes outermost so it sees the complete picture including inner middleware responses

**Key Go concepts:**
- Middleware signature: `func(http.Handler) http.Handler`
- Middleware factory: a function that takes config and returns a middleware
- Struct embedding: embed an interface or struct to inherit all its methods
- Override only what you need — everything else passes through automatically
- `r.WithContext(ctx)` — immutable request pattern, returns new request
- `time.Since(start)` — duration since a recorded start time

**What confused me at first:**
- Why `responseWriter` needs a default status of `http.StatusOK`.
  If a handler writes a body without calling `WriteHeader` explicitly,
  Go uses 200 internally — but our wrapper never sees it. The default covers this case.

**What finally made it clear:**
- Middleware ordering. Drawing the request/response flow as arrows made it clear
  why logging must wrap everything to see the final status and total duration.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Using `context.Background()` in timeout middleware | Breaks the request cancellation chain |
| Putting logging inside timeout middleware | Logging misses status codes written by timeout itself |
| Forgetting default status in `responseWriter` | Handlers that skip `WriteHeader` log status `0` |
| Modifying `r` directly instead of `r.WithContext` | `*http.Request` should be treated as immutable |

---

## 3. Code Demo

### `internal/handler/middleware.go`

```go
package handler

import (
    "context"
    "log"
    "net/http"
    "time"
)

type responseWriter struct {
    http.ResponseWriter
    status int
}

func (rw *responseWriter) WriteHeader(status int) {
    rw.status = status
    rw.ResponseWriter.WriteHeader(status)
}

func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
        next.ServeHTTP(rw, r)
        log.Printf("%s %s %d %s", r.Method, r.URL.Path, rw.status, time.Since(start))
    })
}

func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx, cancel := context.WithTimeout(r.Context(), timeout)
            defer cancel()
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### `cmd/server/main.go` — middleware wired in correct order

```go
r := chi.NewRouter()
r.Use(handler.LoggingMiddleware)
r.Use(handler.TimeoutMiddleware(5 * time.Second))
r.Post("/events", eventHandler.CreateEvent)
```

### Expected log output

```
POST /events 202 243µs
POST /events 400 89µs
```

---

## 4. Interview Takeaway

**What middleware is in Go:**
A function that takes an `http.Handler` and returns a new `http.Handler`.
It runs code before and after the next handler in the chain.
No framework, no annotations — just functions wrapping functions.

**How to capture the response status code:**
`http.ResponseWriter` does not expose the status after it is written.
Wrap it in a struct that embeds `http.ResponseWriter` and overrides `WriteHeader`
to capture the status. Override only what you need — embedding handles the rest.

**Why logging goes outermost:**
Middleware wraps from outside in. The outermost middleware sees the complete
request/response cycle — total duration and final status code including anything
written by inner middleware. Observability middleware always goes outermost.

**Timeout middleware and context:**
Create the child context from `r.Context()` — not `context.Background()`.
Pass it back via `r.WithContext(ctx)` — the request is immutable, this returns a new one.
This connects the HTTP request lifecycle to Go's context cancellation chain.

---

## 5. Cleanup Notes

Day 10 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/handler/middleware.go` — logging and timeout middleware used by all routes
- `cmd/server/main.go` — updated with middleware wiring

**Note:** `log.Printf` in `LoggingMiddleware` will be replaced with `log/slog`
structured logging on Day 20 when configuration is set up.