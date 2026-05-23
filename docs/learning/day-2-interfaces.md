# Day 02 — Structs, Interfaces & the Go Type System

---

## 1. Original Lesson Explanation

### 1.1 Big picture

In Spring Boot you have this pattern constantly:

```java
public interface NotificationService { ... }

@Service
public class EmailNotificationService implements NotificationService { ... }
```

Go has interfaces too — but they work very differently.
Understanding the difference is one of the most important mental shifts in your transition to Go.

---

### 1.2 Why it exists — and why it is different

In Java, a class **declares** that it implements an interface:

```java
public class EmailNotifier implements Notifier { ... }
```

The class knows about the interface. The relationship is explicit and defined upfront.

In Go, **there is no `implements` keyword.** A type satisfies an interface automatically — just by having the right methods.
The type does not need to know the interface exists at all.

```go
type Notifier interface {
    Send(ctx context.Context, event NotificationEvent) error
}

// EmailNotifier never mentions Notifier anywhere
type EmailNotifier struct{}

func (e EmailNotifier) Send(ctx context.Context, event NotificationEvent) error {
    // ...
    return nil
}
```

`EmailNotifier` satisfies `Notifier` automatically. The compiler figures it out.

**Why this matters in practice:**

You can define an interface in one package and a type in a completely separate package — and they work together
without either one knowing about the other. This makes Go code very loosely coupled.

---

### 1.3 The other important difference — interface size

In Java, interfaces tend to be large. A `NotificationService` might have five or six methods.

In Go, the convention is the opposite: **keep interfaces as small as possible.**
One method is normal. Two is fine. Five is a design smell.

The standard library has famous examples:

```go
// io package — just one method
type Writer interface {
    Write(p []byte) (n int, err error)
}
```

Your `Notifier` interface should follow this. One method. That is it.

---

### 1.4 Where to define the interface

This is a Go-specific convention that surprises many Java developers:

> **Define the interface where it is used, not where it is implemented.**

Your dispatcher (the consumer) will depend on `Notifier`.
So `Notifier` should live close to the dispatcher code — in `internal/domain/` —
not inside the `EmailNotifier` package.

This is the opposite of Java, where you typically define the interface in the same package as the implementations.

---

### 1.5 Pointer receiver vs value receiver

A method can be defined on a value or on a pointer to a struct:

```go
// value receiver — gets a copy of the struct
func (e EmailNotifier) Send(...) error { ... }

// pointer receiver — gets the actual struct in memory
func (e *NotifyError) Error() string { ... }
```

**The general rule:**
- Use a **pointer receiver** when the method needs to modify the struct, or when the struct is large
- Use a **value receiver** when the method only reads from the struct and the struct is small
- Be **consistent** within a type — if one method uses a pointer receiver, all methods should

`EmailNotifier` uses a value receiver because it has no fields and `Send` does not modify anything.

---

### 1.6 Compile-time interface check

Go has a useful idiom for verifying that a type satisfies an interface at compile time:

```go
var _ domain.Notifier = notifier.EmailNotifier{}
var _ domain.Notifier = notifier.WebhookNotifier{}
```

The blank identifier `_` means "I don't need this value."
If `EmailNotifier` does not satisfy `Notifier`, the build fails with a clear error.
If it does, these lines do nothing at runtime. This is used in real production Go code.

---

## 2. My Learning Summary

**What I learned:**
- Go interfaces are satisfied implicitly — no `implements` keyword, no declaration needed
- The type does not need to know the interface exists — the compiler checks the method signatures
- Interfaces should be small — one method is normal and idiomatic
- Define interfaces where they are consumed, not where they are implemented
- Pointer receivers and value receivers have different rules and must be consistent within a type

**Key Go concepts:**
- Implicit interface satisfaction — structural typing
- `internal/domain/` as the home for core interfaces and types
- `var _ InterfaceName = ConcreteType{}` — compile-time interface check idiom
- Value receiver vs pointer receiver — copy vs reference

**What confused me at first:**
- In Java the interface is always defined alongside or near its implementations.
  In Go it is defined near the consumer — the opposite direction.

**What finally made it clear:**
- Looking at the import graph: `notifier` imports `domain`, but `domain` does not import `notifier`.
  The dependency flows in one direction only. This is why there is no circular import problem.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Defining the interface in the implementation package | Tight coupling — Go convention is the opposite |
| Large interfaces with many methods | Hard to mock, hard to compose — keep them small |
| Mixing pointer and value receivers on the same type | Can cause subtle interface satisfaction bugs |
| Forgetting the compile-time check | Errors only surface at runtime when the interface is actually used |

---

## 3. Code Demo

### `internal/domain/notifier.go` — the interface lives with the consumer

```go
package domain

import "context"

type Notifier interface {
    Send(ctx context.Context, event NotificationEvent) error
}
```

### `internal/notifier/email.go` — implementation knows nothing about the interface

```go
package notifier

import (
    "context"
    "fmt"

    "github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type EmailNotifier struct{}

func (e EmailNotifier) Send(ctx context.Context, event domain.NotificationEvent) error {
    fmt.Println("sending email notification:", event.ID)
    return nil
}
```

### `internal/notifier/webhook.go` — second implementation, same pattern

```go
package notifier

import (
    "context"
    "fmt"

    "github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type WebhookNotifier struct{}

func (w WebhookNotifier) Send(ctx context.Context, event domain.NotificationEvent) error {
    fmt.Println("sending webhook notification:", event.ID)
    return nil
}
```

### `cmd/server/main.go` — compile-time interface check

```go
var _ domain.Notifier = notifier.EmailNotifier{}
var _ domain.Notifier = notifier.WebhookNotifier{}
```

### Dependency direction

```
notifier → domain
```

`domain` imports only `"context"` — nothing from the project.
`notifier` imports `domain`.
One direction only. No circular imports possible.

---

## 4. Interview Takeaway

**How Go interfaces differ from Java interfaces:**
In Java, a class explicitly declares which interfaces it implements.
In Go, a type satisfies an interface simply by having the right methods — no declaration needed.
The compiler checks the method signatures automatically. This is called structural typing.

**Why small interfaces matter:**
Small interfaces are easier to mock in tests, easier to compose, and easier to satisfy.
The standard library's `io.Writer` and `io.Reader` — both single-method interfaces — are used everywhere
because they are so easy to implement. Your own interfaces should follow the same principle.

**Where to define the interface:**
Define it in the package that uses it, not the package that implements it.
This keeps the dependency flowing in one direction and avoids tight coupling between packages.

**The compile-time check idiom:**
```go
var _ domain.Notifier = notifier.EmailNotifier{}
```
This line does nothing at runtime but fails to compile if `EmailNotifier` does not satisfy `Notifier`.
It is used in production Go code to catch interface mismatches early.

---

## 5. Cleanup Notes

Day 2 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/domain/notifier.go` — the `Notifier` interface is a core domain type used throughout the project
- `internal/notifier/email.go` — `EmailNotifier` is a real notifier implementation
- `internal/notifier/webhook.go` — `WebhookNotifier` is a real notifier implementation
- `cmd/server/main.go` — the compile-time interface checks stay permanently