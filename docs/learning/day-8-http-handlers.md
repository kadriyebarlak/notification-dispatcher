# Day 08 — HTTP Handlers in Go & chi Router

---

## 1. Original Lesson Explanation

### 1.1 Big picture

In Spring Boot you write this:

```java
@RestController
@RequestMapping("/events")
public class EventController {

    @PostMapping
    public ResponseEntity<Void> create(@RequestBody @Valid EventRequest request) {
        return ResponseEntity.accepted().build();
    }
}
```

Spring scans your classpath, finds the annotation, registers the route, deserializes the body,
validates it, and handles the response — all invisibly.

In Go, **you do all of this yourself, explicitly.** There is no classpath scanning, no annotations,
no magic. But because of that, you can see exactly what happens at every step.

---

### 1.2 Why it exists this way

Go's `net/http` package is part of the standard library. It is production-ready on its own.
Many large Go services use nothing but `net/http` for their HTTP layer.

For routing, the standard library's `ServeMux` is basic — it does not handle URL parameters
like `/events/{id}`. So the community uses lightweight routers. The most popular and idiomatic
one is **`chi`**. It is small, uses standard `http.Handler` interfaces, and adds no magic.

There is nothing like Spring's `DispatcherServlet` in Go.
Just a router that maps a path to a handler function.

---

### 1.3 The handler signature

In Go, every HTTP handler is a function with this signature:

```go
func(w http.ResponseWriter, r *http.Request)
```

- `w` — you write the response into this
- `r` — the incoming request, including body, headers, URL

No return value. You write directly to `w`.

```go
func handleCreateEvent(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusAccepted) // 202
}
```

---

### 1.4 Routing with chi

```go
import "github.com/go-chi/chi/v5"

r := chi.NewRouter()
r.Post("/events", handleCreateEvent)

http.ListenAndServe(":8080", r)
```

`chi.NewRouter()` returns a standard `http.Handler`.
It plugs directly into Go's standard `http.ListenAndServe`.

> **Türkçe özet:** Spring'de route'ları annotation ile tanımlarsın.
> Go'da ise bir router'a path ve handler fonksiyonunu kendin bağlarsın.
> Framework yok, annotation yok — sadece fonksiyonlar.

---

### 1.5 Reading JSON from the request body

```go
var req CreateEventRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    http.Error(w, "invalid request body", http.StatusBadRequest)
    return
}
```

`json.NewDecoder` reads the body stream and decodes it into your struct.
If it fails, return 400 immediately. No `@RequestBody`, no automatic validation — explicit.

---

### 1.6 Writing a JSON response

```go
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusAccepted)
json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
```

**Order matters:**
1. Set headers first
2. Call `WriteHeader` second
3. Write body last

Once `WriteHeader` is called, headers are locked. Once the body is written, you cannot change
the status code. This order cannot be reversed.

---

### 1.7 Why `EventHandler` is a struct, not a plain function

A plain function works fine today when the handler has no dependencies.
But in Day 13, the handler will need to call `EventService` to store events in the database:

```go
type EventHandler struct {
    service EventService  // injected dependency
}
```

The struct gives you a home for dependencies injected from `main.go`.
This is Go's equivalent of Spring's constructor injection — manual, explicit, no framework.

Start with a plain function when possible. Add a struct when you have a real dependency to hold.

---

### 1.8 The critical difference from Spring

In Spring, returning `ResponseEntity.badRequest().build()` stops execution — the method returns.

In Go, writing to `w` does **not** stop the function. If you forget `return` after writing
an error response, the function keeps running and writes a second response on top of the first.

Go will log: `superfluous response.WriteHeader call` — but the client already received a corrupted response.

**Always `return` after every error response. No exception.**

---

## 2. My Learning Summary

**What I learned:**
- Go HTTP handlers are plain functions — no annotations, no return values, write directly to `w`
- `chi` is a lightweight idiomatic router that wraps standard `net/http`
- JSON decoding is explicit — `json.NewDecoder(r.Body).Decode(&req)`
- Header → WriteHeader → Body — the order is strict and cannot be reversed
- `EventHandler` is a struct now so it can hold injected dependencies later
- Always `return` after writing an error response — Go does not stop execution for you

**Key Go concepts:**
- `http.ResponseWriter` — write status, headers, and body into this
- `*http.Request` — read body, headers, URL parameters from this
- `json.NewDecoder(r.Body).Decode(&req)` — decode JSON request body
- `json.NewEncoder(w).Encode(v)` — write JSON response body
- `http.Error(w, message, statusCode)` — shorthand for plain text error response
- `w.Header().Set(key, value)` — set response header before WriteHeader
- `chi.NewRouter()` — lightweight idiomatic router
- `log.Fatal(err)` — log error and exit, used for unrecoverable startup failures

**What confused me at first:**
- Why use an `EventHandler` struct instead of a plain function?
  The struct is empty now and feels unnecessary. It will hold the service dependency on Day 13.
  Your handler will need to call EventService to store the event in the database. That dependency has to come from somewhere.
  If you used a plain function, you would have no clean place to hold that dependency. You would either use a global variable — which is bad — or use a closure, which works but is less clean for multiple methods.
  The struct gives you a home for dependencies that will be injected later. This is Go's equivalent of Spring's constructor injection — but manual, explicit, no framework.

**What finally made it clear:**
- In Spring, returning from a controller method stops execution.
  In Go, writing to `w` does not stop the function — you must `return` explicitly after every error.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Setting headers after `WriteHeader` | Headers are locked after `WriteHeader` is called — silently ignored |
| Forgetting `return` after `http.Error` | Function keeps running, second write corrupts the response |
| Ignoring the error from `http.ListenAndServe` | Silent failure if port is already in use |
| Missing JSON struct tags | Go looks for `"Type"` not `"type"` — field never populated from JSON |

---

## 3. Code Demo

### `internal/handler/event.go`

```go
package handler

import (
    "encoding/json"
    "net/http"
)

type EventHandler struct{}

type CreateEventRequest struct {
    Type    string `json:"type"`
    Payload string `json:"payload"`
}

func (h EventHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
    var req CreateEventRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }
    if req.Type == "" {
        http.Error(w, "type is required", http.StatusBadRequest)
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusAccepted)
    json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}
```

### `cmd/server/main.go`

```go
package main

import (
    "fmt"
    "log"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/kadriyebarlak/notification-dispatcher/internal/domain"
    "github.com/kadriyebarlak/notification-dispatcher/internal/handler"
    "github.com/kadriyebarlak/notification-dispatcher/internal/notifier"
)

func main() {
    var _ domain.Notifier = notifier.EmailNotifier{}
    var _ domain.Notifier = notifier.WebhookNotifier{}

    fmt.Println("notification dispatcher starting...")

    eventHandler := handler.EventHandler{}
    r := chi.NewRouter()
    r.Post("/events", eventHandler.CreateEvent)

    fmt.Println("server listening on :8080")
    if err := http.ListenAndServe(":8080", r); err != nil {
        log.Fatal(err)
    }
}
```

### Manual testing with curl

```bash
# 202 — valid request
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{"type":"email","payload":"hello"}'

# 400 — missing type
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{}'

# 400 — invalid JSON
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d 'not-json'
```

---

## 4. Interview Takeaway

**How Go HTTP handlers differ from Spring controllers:**
Go handlers are plain functions — no annotations, no classpath scanning, no framework magic.
You write directly to `http.ResponseWriter`. The framework does not manage the response lifecycle.
This means you are responsible for header order, explicit returns after errors, and error handling.

**Why `chi` instead of the standard library router:**
The standard `ServeMux` does not support URL parameters like `/events/{id}`.
`chi` adds this without introducing a framework — it uses standard `http.Handler` interfaces
and plugs directly into `net/http`. No lock-in, no magic.

**The critical return-after-error rule:**
In Spring, returning `ResponseEntity` stops the method.
In Go, writing to `w` does not stop execution. Always `return` after every `http.Error` call.
Forgetting this causes a second write on top of the first — corrupted response and a runtime warning.

**Why handler structs instead of plain functions:**
Plain functions work for simple handlers with no dependencies.
A struct is used when the handler needs injected dependencies — like a service layer.
The struct holds the dependency; `main.go` injects it at startup.

---

## 5. Cleanup Notes

Day 8 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/handler/event.go` — real HTTP handler, grows through Week 2
- `cmd/server/main.go` — updated with router and server startup