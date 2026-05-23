# Day 09 — Request Validation & JSON Responses

---

## 1. Original Lesson Explanation

### 1.1 Big picture

After Day 8, the handler returns plain text errors via `http.Error`. That works but it is not
production quality. Real APIs return **structured JSON error responses** so clients can parse
them programmatically.

Also, validation only checked if `Type` was empty. A real handler needs to validate more carefully
and return meaningful, consistent errors.

---

### 1.2 Why it matters

In Spring Boot you get this for free with `@Valid` and `BindingResult`.
The framework formats validation errors as JSON automatically.

In Go you do it yourself — but that means you control exactly what the error response looks like.
Every API has a consistent error shape that clients depend on.

A good JSON error response:
```json
{
  "error": "type is required"
}
```

For multiple validation errors:
```json
{
  "errors": ["type is required", "payload is required"]
}
```

---

### 1.3 A reusable response helper

Writing `w.Header().Set(...)`, `w.WriteHeader(...)`, `json.NewEncoder(w).Encode(...)` in every
handler is repetitive and error-prone. Extract a small helper:

```go
func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(v); err != nil {
        log.Printf("failed to encode response: %v", err)
    }
}

func writeError(w http.ResponseWriter, status int, message string) {
    writeJSON(w, status, map[string]string{"error": message})
}
```

Now every handler uses the same consistent response format.
You can never forget to set the Content-Type header.

**Important:** if `Encode` fails after `WriteHeader` has already been called, you cannot change
the status code — it is already sent. Log the error instead of calling `http.Error` again.
Calling `http.Error` after `WriteHeader` triggers a `superfluous response.WriteHeader call` warning.

---

### 1.4 Validating the request

Validation in Go is explicit. No annotation magic. Add a `validate()` method on the request struct:

```go
func (req CreateEventRequest) validate() []string {
    var errs []string
    if req.Type == "" {
        errs = append(errs, "type is required")
    }
    if req.Payload == "" {
        errs = append(errs, "payload is required")
    }
    return errs
}
```

Return a slice of error messages. If the slice is empty, the request is valid.
Collect all errors together — do not fail on the first one.
Clients should know everything that is wrong in a single response.

> **Türkçe özet:** Spring'de `@Valid` annotation'ı validasyonu otomatik yapar.
> Go'da her alanı kendin kontrol eder, hataları bir slice'a toplarsın.
> Daha verbose ama tam olarak ne kontrol ettiğini görürsün.

---

### 1.5 Why `[]string` is limiting — and what to use instead

`validate()` returns `[]string` — plain error messages. This is fine for an MVP.

In a larger API it becomes limiting. If the client receives:
```json
{"errors": ["type is required", "payload is required"]}
```

They must parse the string to figure out which field failed. That is fragile.

The production alternative is a structured validation error type:

```go
type ValidationError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
    Code    string `json:"code"`
}
```

Response becomes:
```json
{
  "errors": [
    {"field": "type", "message": "type is required", "code": "required"},
    {"field": "payload", "message": "payload is required", "code": "required"}
  ]
}
```

Now the client knows exactly which field failed. A UI can highlight the right input.
A client can handle `"code": "required"` differently from `"code": "invalid_format"`
without string matching.

Keep `[]string` for the MVP. Use `[]ValidationError` in a production API.

---

## 2. My Learning Summary

**What I learned:**
- Extract `writeJSON` and `writeError` helpers — consistent responses, no repeated header setup
- Validation belongs on the request struct as a method — handler stays clean
- Collect all validation errors together — never fail on the first error only
- `[]string` is simple but limiting — structured `ValidationError` is the production pattern
- After `WriteHeader` is called, the status code is locked — log encode errors, do not re-write

**Key Go concepts:**
- `json.NewEncoder(w).Encode(v)` — write JSON directly to response writer
- `map[string]string{"error": message}` — simple inline response struct
- `map[string][]string{"errors": errs}` — multiple validation errors in one response
- `any` — Go 1.18+ alias for `interface{}`, used for generic function parameters
- `append(errs, "message")` — building an error slice without pre-allocation

**What confused me at first:**
- Why not call `http.Error` when `json.Encode` fails?
  Because `WriteHeader` was already called — the status code is locked.
  A second write triggers a `superfluous response.WriteHeader call` warning.
  Log the error instead.

**What finally made it clear:**
- The response lifecycle is one-way: headers → status → body.
  Once you move forward, you cannot go back.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Calling `http.Error` after `WriteHeader` | Status already sent — superfluous WriteHeader warning |
| Failing on the first validation error only | Client makes multiple round trips to fix all errors |
| Repeating `w.Header().Set(...)` in every handler | Error-prone — extract to a helper |
| Using `[]string` for validation errors in a real API | Client must parse strings to identify which field failed |

---

## 3. Code Demo

### `internal/handler/response.go` — reusable response helpers

```go
package handler

import (
    "encoding/json"
    "log"
    "net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(v); err != nil {
        log.Printf("failed to encode response: %v", err)
    }
}

func writeError(w http.ResponseWriter, status int, message string) {
    writeJSON(w, status, map[string]string{"error": message})
}
```

### `internal/handler/event.go` — updated handler with structured validation

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

func (req CreateEventRequest) validate() []string {
    var errs []string
    if req.Type == "" {
        errs = append(errs, "type is required")
    }
    if req.Payload == "" {
        errs = append(errs, "payload is required")
    }
    return errs
}

func (h EventHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
    var req CreateEventRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if errs := req.validate(); len(errs) > 0 {
        writeJSON(w, http.StatusBadRequest, map[string][]string{
            "errors": errs,
        })
        return
    }
    writeJSON(w, http.StatusAccepted, map[string]string{
        "status": "accepted",
    })
}
```

### Manual testing with curl

```bash
# 202 — valid request
curl -s -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{"type":"email","payload":"hello"}' | jq

# 400 — both fields missing
curl -s -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{}' | jq

# 400 — only type missing
curl -s -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{"payload":"hello"}' | jq
```

---

## 4. Interview Takeaway

**How Go validation differs from Spring:**
Spring uses `@Valid` and `BindingResult` — the framework handles validation automatically.
In Go, validation is explicit code — a method on the request struct that returns errors.
More verbose, but you control exactly what is validated and what the error looks like.

**Why collect all errors instead of failing on the first:**
Clients should know everything wrong with a request in a single response.
Failing on the first error forces multiple round trips — bad UX and bad API design.

**Why `[]ValidationError` over `[]string` in production:**
Plain strings require clients to parse message text to identify which field failed.
A structured type with `field`, `message`, and `code` lets clients handle errors
programmatically — highlight the right form field, handle specific error codes differently.
```go
type ValidationError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
    Code    string `json:"code"`
}
```

**The response lifecycle rule:**
Headers → WriteHeader → Body. One direction only.
Once `WriteHeader` is called, the status code is locked.
Once the body starts writing, you cannot change anything.
Always log encode errors — never try to write a second response.

---

## 5. Cleanup Notes

Day 9 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/handler/response.go` — `writeJSON` and `writeError` used by all handlers
- `internal/handler/event.go` — updated with structured validation and response helpers