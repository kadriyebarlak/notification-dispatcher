# Day 06 — Go Testing Basics & Table-Driven Tests

---

## 1. Original Lesson Explanation

### 1.1 Big picture

In Java you use JUnit. You annotate test classes, use `@Test`, `@BeforeEach`, assertion libraries
like AssertJ, and a test runner handles everything.

In Go, **testing is built into the language.** No external framework, no annotations, no test runner
to install. Just files ending in `_test.go` and the `go test` command. That is the entire setup.

---

### 1.2 Why it exists

Go's designers wanted testing to be a first-class feature — not something you bolt on with a framework.
The result is a testing system that is simple, fast, and consistent across every Go project.
When you open any Go codebase, the tests always look the same.

---

### 1.3 The basics

**Test files** live next to the code they test, in the same package:
```
internal/domain/event.go
internal/domain/event_test.go   ← test file
```

**Test functions** start with `Test`, take `*testing.T`, and nothing else:
```go
func TestSomething(t *testing.T) {
    // test body
}
```

**Failing a test:**
```go
t.Errorf("got %v, want %v", got, want)  // marks failed, continues
t.Fatalf("got %v, want %v", got, want)  // marks failed, stops immediately
```

**Running tests:**
```bash
go test ./...           # run all tests in the project
go test ./internal/...  # run tests in a specific path
go test -v ./...        # verbose — shows each test name and result
go test -race ./...     # run with the race detector.-race Go’nun data race detector.aynı veriye aynı anda birden fazla goroutine erişirse ve bu erişimlerden en az biri write ise, Go bunu yakalayabilsin diye
```

No Maven, no Gradle, no plugin configuration.

---

### 1.4 Table-driven tests

This is the most important Go testing pattern. You will use it everywhere.

Instead of writing one test function per case, you define a **slice of test cases** and loop over them:

```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name string
        a, b int
        want int
    }{
        {"positive numbers", 2, 3, 5},
        {"zero", 0, 0, 0},
        {"negative", -1, -2, -3},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Add(tt.a, tt.b)
            if got != tt.want {
                t.Errorf("Add(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
            }
        })
    }
}
```

`t.Run` creates a **subtest** with its own name. When a subtest fails, you see exactly which case failed:
```
--- FAIL: TestAdd/negative (0.00s)
```

Adding a new test case is one line in the slice. No new function, no new annotation.

> **Türkçe özet:** Her test durumu için ayrı fonksiyon yazmak yerine, test case'leri bir slice içinde
> tanımlarsın ve döngüyle geçersin. Yeni bir case eklemek tek satır.
> Go'da bu o kadar yaygın ki neredeyse bir kural gibi.

---

### 1.5 Test package: `package domain` vs `package domain_test`

Go allows two approaches in the same test file:

**`package domain`** — same package as the code being tested.
- Can access unexported (lowercase) identifiers
- Used for testing internal implementation details

**`package domain_test`** — external test package in the same folder.
- Can only access exported (uppercase) identifiers
- Tests the package the same way any external caller would use it
- Prevents tests from depending on internal details you might later refactor

Both can coexist in the same folder. Most Go projects use both.

---

### 1.6 Why test files live next to production code

In Java, test classes live in a parallel `src/test/java` tree.
In Go, test files sit right next to the production code in the same folder.

**Advantage:** When you open `errors.go` you immediately see `errors_test.go` alongside it.
No parallel tree to navigate. The test is always one file away from the code it covers.

**Second advantage:** `package domain` test files can access unexported identifiers naturally —
no reflection, no package-private tricks needed.

---

## 2. My Learning Summary

**What I learned:**
- Go testing needs no framework — just `_test.go` files and `go test`
- Table-driven tests use a slice of structs and a loop — the standard Go pattern
- `t.Run` creates subtests with names so failures are easy to identify
- Test files live next to production code — easier to find, easier to maintain
- `package domain` vs `package domain_test` — internal access vs public API testing

**Key Go concepts:**
- `_test.go` suffix — tells the Go toolchain this is a test file
- `func TestXxx(t *testing.T)` — test function signature, no annotations
- `t.Errorf` / `t.Fatalf` — fail with message, continue vs stop
- `t.Run(name, func)` — creates a named subtest
- `go test -v ./...` — verbose output showing all subtest results
- `package foo_test` — external test package, only exported API accessible

**What confused me at first:**
- The difference between `package domain` and `package domain_test` in a test file.
  Both live in the same folder — Go allows them to coexist.

**What finally made it clear:**
- `package domain` = I am inside the package, I can see everything including unexported.
  `package domain_test` = I am a user of the package, I can only see what is exported.
  Use internal for testing implementation details. Use external for testing the public API.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Using `t.Fatalf` for every assertion | Stops the test immediately — other cases in the table never run |
| Not using `t.Run` in table tests | All cases collapse into one test — hard to identify which case failed |
| Testing only the happy path | Edge cases like empty fields are where real bugs hide |
| Using `package domain_test` when you need unexported access | Compile error — external package cannot see unexported identifiers |

---

## 3. Code Demo

### `internal/domain/errors_test.go` — testing `NotifyError.Error()`

```go
package domain

import "testing"

func TestNotifyError_Error(t *testing.T) {
    tests := []struct {
        name string
        err  *NotifyError
        want string
    }{
        {
            name: "with event ID and reason",
            err:  &NotifyError{EventID: "evt-001", Reason: "timeout"},
            want: "notification failed for event evt-001: timeout",
        },
        {
            name: "with empty reason",
            err:  &NotifyError{EventID: "evt-001", Reason: ""},
            want: "notification failed for event evt-001: ",
        },
        {
            name: "with empty event ID",
            err:  &NotifyError{EventID: "", Reason: "connection refused"},
            want: "notification failed for event : connection refused",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := tt.err.Error()
            if got != tt.want {
                t.Errorf("got %q, want %q", got, tt.want)
            }
        })
    }
}
```

### `internal/domain/event_test.go` — testing `EventStatus` constants

```go
package domain

import "testing"

func TestEventStatusConstants(t *testing.T) {
    tests := []struct {
        name   string
        status EventStatus
        want   string
    }{
        {"pending status", StatusPending, "pending"},
        {"delivered status", StatusDelivered, "delivered"},
        {"failed status", StatusFailed, "failed"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := string(tt.status)
            if got != tt.want {
                t.Errorf("got %q, want %q", got, tt.want)
            }
        })
    }
}
```

### Running tests

```bash
go test -v ./internal/domain/...
```

Expected output:
```
--- PASS: TestNotifyError_Error/with_event_ID_and_reason
--- PASS: TestNotifyError_Error/with_empty_reason
--- PASS: TestNotifyError_Error/with_empty_event_ID
--- PASS: TestEventStatusConstants/pending_status
--- PASS: TestEventStatusConstants/delivered_status
--- PASS: TestEventStatusConstants/failed_status
```

### Why testing `EventStatus` constants matters

Your DB stores `"pending"`. Your JSON responses return `"pending"`.
If someone refactors the constant to `"PENDING"`, everything breaks silently at runtime.
This test catches it immediately.

---

## 4. Interview Takeaway

**How Go testing differs from Java:**
No JUnit, no annotations, no test runner to configure.
Testing is built into the language — `_test.go` files and `go test` are all you need.
Every Go project uses the same pattern, so any Go codebase is immediately familiar.

**What table-driven tests are and why they matter:**
A slice of test cases iterated with a loop. Adding a new case is one line.
`t.Run` gives each case a name so failures are easy to identify.
This is the standard Go pattern — interviewers expect to see it in your code.

**`package foo` vs `package foo_test`:**
`package foo` — same package, can access unexported identifiers, used for internal tests.
`package foo_test` — external package, only exported API, tests the package as a real caller would.
Prefer `package foo_test` for public API tests — it prevents tests from coupling to internal details.

**Why test files live next to production code:**
Proximity makes tests easier to find and maintain.
No parallel directory tree to navigate.
`package foo` tests can naturally access unexported identifiers without reflection tricks.

---

## 5. Cleanup Notes

Day 6 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/domain/errors_test.go` — real tests, stay in the project
- `internal/domain/event_test.go` — real tests, stay in the project

**Week 1 is complete.** All six days are committed. Week 2 begins building the HTTP API.