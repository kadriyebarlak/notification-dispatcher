# Day 23 — HTTP Handler Tests with Body Assertions

---

## 1. Original Lesson Explanation

### 1.1 Big picture

On Day 14, basic handler tests were written for `CreateEvent` — three cases, status code only.
Day 23 makes them production-quality:

- Assert on the **response body**, not just the status code
- Add more edge cases to `CreateEvent`
- Write tests for `ListEvents`
- Verify the `Content-Type` header is set correctly

---

### 1.2 Asserting on response body

`httptest.ResponseRecorder` captures the full response — status, headers, and body:

```go
rec := httptest.NewRecorder()
handler.CreateEvent(rec, req)

// status code
if rec.Code != http.StatusAccepted {
    t.Errorf(...)
}

// response body
var body map[string]string
if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
    t.Fatalf("could not decode response body: %v", err)
}
if body["status"] != "accepted" {
    t.Errorf("got body status %q, want %q", body["status"], "accepted")
}

// content type header
if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
    t.Errorf("got Content-Type %q, want %q", ct, "application/json")
}
```

---

### 1.3 Where to test `Content-Type`

`Content-Type: application/json` is set inside `writeJSON` — not in the handler itself.

**Test `writeJSON` directly** for the contract — one focused test, one place.

**Test it in one handler test case** — the happy path — to confirm the handler
actually uses `writeJSON` and the header reaches the client.

**Skip it in error and validation cases** — they already verify the status code.
Checking `Content-Type` in every case adds noise with no new information.

> **Principle:** test behaviour at the level where it is defined.
> Verify integration at the level where it is used — but only once.

---

### 1.4 Verifying service arguments with `calledStatus`

For `ListEvents`, it is not enough to check the response.
You also need to verify the handler called the service with the correct argument:

```go
type fakeEventService struct {
    listEvents   []domain.NotificationEvent
    listErr      error
    calledStatus string  // records what status was passed
}

func (f *fakeEventService) ListByStatus(ctx context.Context, status string) ([]domain.NotificationEvent, error) {
    f.calledStatus = status  // capture the argument
    return f.listEvents, f.listErr
}
```

Then assert on it:
```go
if service.calledStatus != tt.wantCalledStatus {
    t.Errorf("got called status %q, want %q", service.calledStatus, tt.wantCalledStatus)
}
```

This catches bugs where the handler passes the wrong status or forgets to default to `"pending"`.

---

### 1.5 Table design for optional assertions

Use pointer or nil-able fields for assertions that only apply to some cases:

```go
tests := []struct {
    name            string
    body            string
    mockErr         error
    wantStatus      int
    wantBody        map[string]string  // nil = skip body assertion
    wantContentType string             // empty = skip header assertion
}
```

Guard each optional assertion:
```go
if tt.wantBody != nil {
    // decode and assert
}
if tt.wantContentType != "" {
    // assert header
}
```

This keeps cases clean — only set what a specific case actually cares about.

---

## 2. My Learning Summary

**What I learned:**
- `rec.Body` — decode with `json.NewDecoder` to assert on response content
- `rec.Header().Get("Content-Type")` — assert on response headers
- `calledStatus` on the fake — verify the handler passes the correct argument to the service
- Table design with nil-able fields — optional assertions without noise in every case
- `t.Fatalf` on decode failure — subsequent assertions would be invalid on bad JSON
- Early `return` after error case — no attempt to decode events when error is expected

**Key Go concepts:**
- `rec.Body` is `*bytes.Buffer` — readable with `json.NewDecoder`
- `rec.Header()` returns `http.Header` — same as real response headers
- Ranging over `wantBody` map — flexible body assertions without hardcoded keys
- Recording service arguments in fakes — not just return values but input verification

**What was done well beyond the lesson:**
- `calledStatus` field on the fake — verifies the service receives the right input,
  not just that it was called. Catches bugs in argument passing.
- Ranging over `wantBody` keys — adding a new body assertion is one line in the table.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Only asserting status code | Misses bugs in response body format and content |
| Checking `Content-Type` in every test case | Noise — test the contract once, verify integration once |
| Not recording service arguments in fakes | Cannot verify the handler passes correct values |
| Using `t.Errorf` before JSON decode | Test continues with empty/nil body — misleading failures |
| Not testing the default status case | The defaulting logic is a real code path that can break |

---

## 3. Code Demo

### Updated `fakeEventService` with argument recording

```go
type fakeEventService struct {
    createErr    error
    listEvents   []domain.NotificationEvent
    listErr      error
    calledStatus string
}

func (f *fakeEventService) Create(ctx context.Context, eventType, payload string) error {
    return f.createErr
}

func (f *fakeEventService) ListByStatus(ctx context.Context, status string) ([]domain.NotificationEvent, error) {
    f.calledStatus = status
    return f.listEvents, f.listErr
}
```

### `TestCreateEvent` — table with optional body and header assertions

```go
tests := []struct {
    name            string
    body            string
    mockErr         error
    wantStatus      int
    wantBody        map[string]string
    wantContentType string
}{
    {
        name:            "valid request",
        body:            `{"type":"email","payload":"hello"}`,
        wantStatus:      http.StatusAccepted,
        wantBody:        map[string]string{"status": "accepted"},
        wantContentType: "application/json",
    },
    {
        name:       "missing type",
        body:       `{"payload":"hello"}`,
        wantStatus: http.StatusBadRequest,
    },
    {
        name:       "missing payload",
        body:       `{"type":"email"}`,
        wantStatus: http.StatusBadRequest,
    },
    {
        name:       "missing type and payload",
        body:       `{}`,
        wantStatus: http.StatusBadRequest,
    },
    {
        name:       "invalid json",
        body:       `{"type":`,
        wantStatus: http.StatusBadRequest,
    },
    {
        name:            "service error",
        body:            `{"type":"email","payload":"hello"}`,
        mockErr:         errors.New("database down"),
        wantStatus:      http.StatusInternalServerError,
        wantBody:        map[string]string{"error": "failed to create event"},
        wantContentType: "application/json",
    },
}
```

### `TestListEvents` — verifying service argument and event count

```go
{
    name: "returns events for given status",
    url:  "/events?status=failed",
    listEvents: []domain.NotificationEvent{
        {ID: "evt-001", Type: domain.EventTypeEmail, Status: domain.StatusFailed},
        {ID: "evt-002", Type: domain.EventTypeWebhook, Status: domain.StatusFailed},
    },
    wantStatus:       http.StatusOK,
    wantCalledStatus: string(domain.StatusFailed),
    wantEventCount:   2,
},
{
    name:             "defaults to pending when no status given",
    url:              "/events",
    listEvents:       []domain.NotificationEvent{{ID: "evt-003"}},
    wantStatus:       http.StatusOK,
    wantCalledStatus: string(domain.StatusPending),
    wantEventCount:   1,
},
{
    name:             "service error returns 500",
    url:              "/events?status=pending",
    listErr:          errors.New("database down"),
    wantStatus:       http.StatusInternalServerError,
    wantCalledStatus: string(domain.StatusPending),
    wantError:        "failed to list events",
},
```

---

## 4. Interview Takeaway

**What `httptest` enables:**
Testing HTTP handlers completely in isolation — no running server, no network.
`ResponseRecorder` captures status, headers, and body exactly as a real client would see them.
Tests run in milliseconds and are fully deterministic.

**Why assert on response body, not just status code:**
Status code tells you the category of response. Body tells you the content.
A handler can return 200 with an empty body, wrong format, or incorrect field names.
Body assertions catch these bugs. Status-only tests miss them.

**Recording service arguments in fakes:**
Fakes are not just for controlling return values — they can record input too.
`calledStatus string` captures what the handler passed to the service.
This verifies the handler's logic, not just the service's response.

**Where to test cross-cutting concerns like `Content-Type`:**
Test the contract where it is defined — a focused `writeJSON` test.
Verify integration where it is used — one handler test case.
Do not repeat in every case — that is noise, not coverage.

---

## 5. Cleanup Notes

Day 23 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/handler/event_test.go` — expanded with body assertions and `ListEvents` tests