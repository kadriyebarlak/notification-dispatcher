# Day 01 — Project Setup & Go Module Structure

---

## 1. Original Lesson Explanation

### 1.1 Big picture

In Spring Boot, the framework decides a lot for you. You add a dependency, annotate a class, and it works.
The structure is mostly dictated by Maven/Gradle conventions and Spring's expectations.

Go is the opposite. **There is no framework telling you where to put things.**
You decide the structure. The community has settled on a set of conventions — not enforced by the language,
but widely used in real Go projects. Your job on Day 1 is to understand those conventions
and set up your project correctly from day one.

---

### 1.2 Why it exists

Go projects grow complex fast if you don't separate concerns early.
The standard layout solves three real problems:

**`cmd/`** — Your project might have more than one runnable binary later
(e.g. a server and a CLI migration tool). Each goes in its own folder under `cmd/`.
Each has its own `main.go`. This is where execution starts — nothing else goes here.

**`internal/`** — This is a Go-enforced rule, not just a convention.
Code inside `internal/` **cannot be imported by other Go modules**.
It is your private application code. Putting your domain logic, handlers, and services here
prevents accidental reuse and signals "this is not a public library."

**`pkg/`** — Optional. Used for code that could be reused by external projects.
For your project, you may not need this at all. Don't create it unless you have a reason.

**`go.mod`** — This is Go's equivalent of `pom.xml` or `build.gradle`.
It defines your module name and Go version. Every Go project starts with this file.

---

### 1.3 What the project structure looks like

```
notification-dispatcher/
├── cmd/
│   └── server/
│       └── main.go          ← entry point, wires everything together
├── internal/
│   ├── domain/              ← core types (NotificationEvent, Notifier interface)
│   ├── handler/             ← HTTP handlers
│   ├── service/             ← business logic
│   └── repository/          ← database layer
├── migrations/              ← SQL migration files (week 2)
├── .env.example
├── Makefile
├── go.mod
└── README.md
```

Not all of this is filled on Day 1. The skeleton is created so that every future day
has a clean home for its code.

---

### 1.4 Three things to know before starting

**`go mod init`** creates your `go.mod`. The module name is usually your GitHub path:
```bash
go mod init github.com/yourusername/notification-dispatcher
```

**`main.go` in `cmd/server/`** is your entry point. It should be thin — just startup code.
No business logic here.

**`internal/` is enforced by the Go compiler.** If you try to import `internal/` from outside
the module, it will refuse to compile. This is a feature, not a limitation.

---

### 1.5 Domain types — structs and custom types

The first domain type for this project is `NotificationEvent`:

```go
type NotificationEvent struct {
    ID         string
    Type       string
    Payload    string
    Status     EventStatus
    RetryCount int
}
```

Go has no enums. The idiomatic pattern is a **named string type with constants**:

```go
type EventStatus string

const (
    StatusPending   EventStatus = "pending"
    StatusDelivered EventStatus = "delivered"
    StatusFailed    EventStatus = "failed"
)
```

**Java comparison:**

| Java | Go |
|---|---|
| `enum EventStatus { PENDING, DELIVERED, FAILED }` | `type EventStatus string` + constants |
| Compiler prevents invalid values | Any string can be assigned — less strict |
| Needs conversion for JSON/DB | Works naturally with JSON and DB values |

The Go approach is simpler and lighter. The tradeoff is that the compiler does not protect you
from invalid values — a bad string from a JSON payload or a database row will compile and run.

---

### 1.6 Makefile

A simple `Makefile` is the standard way to define project commands in Go.
No Maven plugins, no Gradle tasks — just `make` targets:

```makefile
run:
    go run ./cmd/server
```

Run with: `make run`

---

## 2. My Learning Summary

**What I learned:**
- Go has no framework — project structure is a community convention, not enforced by a tool
- `cmd/` is for entry points, `internal/` is for private application code
- `internal/` is compiler-enforced — code there cannot be imported by outside modules
- `go.mod` is the Go equivalent of `pom.xml` or `build.gradle`
- Named string types with constants are Go's idiomatic replacement for Java enums

**Key Go concepts:**
- `go mod init` — initialises the module and creates `go.mod`
- `cmd/server/main.go` — thin entry point, wires everything, no business logic
- `internal/` — private by compiler rule, not just by convention
- Named type `type EventStatus string` — idiomatic enum replacement
- `Makefile` — standard project command runner in Go

**What confused me at first:**
- In Java, enum prevents assigning invalid values at compile time.
  In Go, `type EventStatus string` does not — any string is assignable.

**What finally made it clear:**
- Go trades compiler strictness for simplicity and JSON/DB compatibility.
  The named type still gives you meaningful constant names and type safety at the function signature level —
  you just don't get exhaustive value checking.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Putting business logic in `main.go` | Entry point should only wire and start — nothing else |
| Creating `pkg/` without a reason | Adds noise — only create it if you have reusable public code |
| Using plain `string` for status fields | Loses type safety at function boundaries |
| Forgetting `RetryCount` in the domain struct early | Harder to add later when multiple files depend on the struct |

---

## 3. Code Demo

### `go.mod` — module definition

```
module github.com/kadriyebarlak/notification-dispatcher

go 1.22
```

### `internal/domain/event.go` — domain struct and status type

```go
package domain

type NotificationEvent struct {
    ID         string
    Type       string
    Payload    string
    Status     EventStatus
    RetryCount int
}

type EventStatus string

const (
    StatusPending   EventStatus = "pending"
    StatusDelivered EventStatus = "delivered"
    StatusFailed    EventStatus = "failed"
)
```

### `cmd/server/main.go` — thin entry point

```go
package main

import "fmt"

func main() {
    fmt.Println("notification dispatcher starting...")
}
```

### `Makefile` — project commands

```makefile
run:
    go run ./cmd/server

build:
    go build -o bin/server ./cmd/server
```

### Running the project

```bash
make run
# output: notification dispatcher starting...
```

---

## 4. Interview Takeaway

**How Go project structure differs from Spring Boot:**
Spring Boot dictates structure through conventions and classpath scanning.
Go has no framework — you choose the structure. The `cmd/` + `internal/` layout is a community
convention followed by most production Go projects, not a language requirement (except for `internal/`).

**What `internal/` means:**
The Go compiler enforces that packages inside `internal/` cannot be imported by code outside the module.
It is a hard boundary for private application code — not just a naming hint.

**Go's enum replacement:**
Go has no enum keyword. The idiomatic pattern is a named string type with typed constants:
```go
type EventStatus string
const StatusPending EventStatus = "pending"
```
Advantage: simple, works naturally with JSON and databases.
Disadvantage: the compiler does not prevent assigning arbitrary string values.

**Why `main.go` should be thin:**
The entry point is the composition root — it wires dependencies and starts the server.
Putting business logic there makes the code untestable and hard to maintain.

---

## 5. Cleanup Notes

Day 1 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `go.mod` — module definition, always present
- `internal/domain/event.go` — `NotificationEvent` and `EventStatus` are core domain types
- `cmd/server/main.go` — entry point, grows over the 30 days
- `Makefile` — project commands, grows over the 30 days