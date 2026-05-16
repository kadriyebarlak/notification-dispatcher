# Day 03 — Error Handling: The Go Way

---

## 1. Original Lesson Explanation

### 1.1 Big picture

In Java, when something goes wrong, you throw an exception. The caller either catches it or lets it bubble up.
The language has a dedicated mechanism for this.

Go has no exceptions. **Errors are just values.** A function that can fail returns an `error` as its last return value.
The caller checks it immediately. If you ignore it, the compiler does not complain — but you will regret it in production.

```go
result, err := doSomething()
if err != nil {
    // handle it
}
```

This feels repetitive at first. It is. But it makes error flow completely explicit — you always know exactly where an error can occur and where it is handled.

---

### 1.2 Why it exists

Java exceptions have a hidden cost: they can come from anywhere and travel invisibly up the call stack.
Go's designers considered this a source of bugs and complexity.
Making errors explicit return values forces you to think about failure at every step.

> **Türkçe özet:** Java'da hata fırlatırsın ve birileri yakalar — arada ne olduğu belirsiz olabilir.
> Go'da hata bir return value'dur, her fonksiyon kendi hatasını döner ve caller hemen kontrol eder.
> Daha verbose ama daha şeffaf.

---

### 1.3 The three patterns you need to know

**Pattern 1 — Simple sentinel error**
```go
import "errors"

var ErrNotFound = errors.New("event not found")
```
Use this for fixed, comparable error values.
The caller can check against this exact value using `errors.Is`.

---

**Pattern 2 — Wrapping errors with context**
```go
fmt.Errorf("failed to send notification: %w", err)
```
The `%w` verb wraps the original error inside a new one.
The caller gets a meaningful message, but the original error is still there underneath.

This is equivalent to Java's `throw new RuntimeException("message", cause)` — but as a return value.

---

**Pattern 3 — Custom error type**
```go
type NotifyError struct {
    EventID string
    Reason  string
}

func (e *NotifyError) Error() string {
    return fmt.Sprintf("notification failed for event %s: %s", e.EventID, e.Reason)
}
```
Any type that has an `Error() string` method satisfies the built-in `error` interface.
That is all Go needs. No `extends Exception`, no special keywords.

---

### 1.4 Checking errors — `errors.Is` and `errors.As`

**`errors.Is`** — checks if an error *is* a specific sentinel value, even if it has been wrapped:
```go
if errors.Is(err, ErrNotFound) {
    // handle not found
}
```

**`errors.As`** — checks if an error *is of a specific type*, and if so, extracts it:
```go
var notifyErr *NotifyError
if errors.As(err, &notifyErr) {
    fmt.Println("failed event ID:", notifyErr.EventID)
}
```

**Why not just use `==` or a type assertion?**

Because of wrapping. If someone wrapped your error with `fmt.Errorf("...: %w", err)`,
a plain `==` check will fail. `errors.Is` and `errors.As` unwrap the chain automatically
and check at every level.

> **Türkçe özet:** `errors.Is` "bu hata şu spesifik hata mı?" diye sorar.
> `errors.As` "bu hata şu tipte mi, öyleyse çıkar" der.
> İkisi de wrap edilmiş zincirleri otomatik açar. `==` ile karşılaştırma yaparsan wrap edilmiş hataları kaçırırsın.

---

## 2. My Learning Summary

**What I learned:**
- Go has no exceptions — errors are return values, always explicit
- Three patterns: sentinel errors, wrapped errors with `%w`, custom error types
- `errors.Is` and `errors.As` unwrap error chains automatically — plain `==` does not
- The correct check order: `errors.Is` first, then `errors.As`, then unknown fallthrough

**Key Go concepts:**
- `errors.New` — creates a simple sentinel error
- `fmt.Errorf("%w", err)` — wraps an error with additional context
- `errors.Is` — checks identity through wrapping
- `errors.As` — checks type and extracts through wrapping
- Pointer receiver on `Error() string` — means you must return `&NotifyError{}`, not `NotifyError{}`

**What confused me at first:**
- The difference between pointer receiver and value receiver on the `Error()` method.
  Returning `NotifyError{}` instead of `&NotifyError{}` would cause `errors.As` to silently fail.

**What finally made it clear:**
- If the `Error()` method has a pointer receiver `(e *NotifyError)`, then the interface is only satisfied by a pointer.
  `errors.As` checks against `*NotifyError`, not `NotifyError`. Returning a value instead of a pointer breaks the match silently.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Using `==` to compare wrapped errors | Wrapping breaks equality — use `errors.Is` |
| Returning `NotifyError{}` with a pointer receiver | `errors.As` will not match — return `&NotifyError{}` |
| Ignoring errors with `_` | Silent failures in production |
| Checking `errors.As` before `errors.Is` | Sentinel errors should be checked first |

---

## 3. Code Demo

### `internal/domain/errors.go` — sentinel error and custom error type

```go
package domain

import (
    "errors"
    "fmt"
)

var ErrEventNotFound = errors.New("event not found")

type NotifyError struct {
    EventID string
    Reason  string
}

func (e *NotifyError) Error() string {
    return fmt.Sprintf("notification failed for event %s: %s", e.EventID, e.Reason)
}
```

### `simulateDispatch` — returning different error types

```go
func simulateDispatch(eventID string) error {
    if eventID == "" {
        return fmt.Errorf("simulateDispatch: %w", domain.ErrEventNotFound)
    }

    if eventID == "fail" {
        return &domain.NotifyError{
            EventID: eventID,
            Reason:  "connection refused",
        }
    }

    return nil
}
```

### Checking errors in the correct order

```go
eventIDs := []string{"", "fail", "evt-001"}

for _, eventID := range eventIDs {
    err := simulateDispatch(eventID)
    if err == nil {
        fmt.Println("dispatch succeeded for event:", eventID)
        continue
    }

    if errors.Is(err, domain.ErrEventNotFound) {
        fmt.Println("event not found:", err)
        continue
    }

    var notifyErr *domain.NotifyError
    if errors.As(err, &notifyErr) {
        fmt.Println("notify error:")
        fmt.Println("event id:", notifyErr.EventID)
        fmt.Println("reason:", notifyErr.Reason)
        continue
    }

    fmt.Println("unknown error:", err)
}
```

### Wrapping confirmation

Wrapping with `%w` does not break `errors.Is`:
```go
wrapped := fmt.Errorf("simulateDispatch: %w", domain.ErrEventNotFound)
fmt.Println(errors.Is(wrapped, domain.ErrEventNotFound)) // true
```

---

## 4. Interview Takeaway

**What Go error handling is:**
Errors are plain values returned as the last return value of a function.
There are no exceptions, no try/catch blocks. Every error is handled explicitly at the call site.

**Why `errors.Is` and `errors.As` instead of `==`:**
Errors are often wrapped with `fmt.Errorf("%w", err)` to add context.
Plain `==` comparison breaks through wrapping. `errors.Is` and `errors.As` unwrap the chain
at every level and check correctly.

**When to use each pattern:**
- Sentinel error (`errors.New`) — when the caller needs to identify a specific condition, like not found
- Wrapped error (`fmt.Errorf("%w")`) — when you want to add context but preserve the original error
- Custom error type — when the caller needs structured data from the error, like an event ID or reason

**Pointer receiver on `Error() string`:**
If `Error()` has a pointer receiver, the `error` interface is only satisfied by a pointer to the type.
Return `&NotifyError{}`, not `NotifyError{}`. Returning a value type will cause `errors.As` to silently fail.

---

## 5. Cleanup Notes

Day 3 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/domain/errors.go` — `ErrEventNotFound` and `NotifyError` are real domain types used throughout the project
- `cmd/server/main.go` — the `simulateDispatch` function and the error checking loop were temporary demo code

**Clean up:**
- `cmd/server/main.go` — the `simulateDispatch` function and the event loop added during Day 3 were demo code.
  They were replaced by the context demo on Day 4 and should be removed when `main.go` is cleaned up after Day 4.