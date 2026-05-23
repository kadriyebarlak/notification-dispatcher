# Day 11 — Database Layer with PostgreSQL & pgx

---

## 1. Original Lesson Explanation

### 1.1 Big picture

In Spring Boot you define a `JpaRepository` interface, annotate your entity, and Spring Data
generates the SQL for you. You rarely write raw SQL.

In Go, **you write the SQL yourself.** No ORM, no generated queries, no annotations.
You use a database driver directly. This sounds like more work — but it means you always know
exactly what query is running. No surprises, no N+1 problems hiding behind an abstraction.

---

### 1.2 Why `pgx` instead of `database/sql`

Go's standard library has `database/sql` — a generic interface that works with any database.
It is fine but basic.

`pgx` is a PostgreSQL-specific driver that is faster, supports more PostgreSQL features,
and has better error handling. It is the standard choice for PostgreSQL in Go.

Use `pgx/v5@v5.7.2` with Go 1.23 — versions above `v5.7.x` require Go 1.25+.

---

### 1.3 The repository pattern in Go

Same concept as Spring Data — but manual and interface-based:

```go
type EventRepository interface {
    Insert(ctx context.Context, event NotificationEvent) error
    FindByStatus(ctx context.Context, status EventStatus) ([]NotificationEvent, error)
    UpdateStatus(ctx context.Context, id string, status EventStatus, retryCount int) error
}
```

The interface lives in `internal/domain/` — the consumer side.
The PostgreSQL implementation lives in `internal/repository/`.
The service depends on the interface, not the implementation.
Same dependency inversion as `Notifier` on Day 2.

> **Türkçe özet:** Spring Data'da interface tanımlarsın, framework implement eder.
> Go'da interface'i kendin tanımlarsın, implementasyonu da kendin yazarsın.
> Daha fazla kod ama tam olarak ne çalıştığını görürsün.

---

### 1.4 Connecting to PostgreSQL with `pgxpool`

```go
import "github.com/jackc/pgx/v5/pgxpool"

pool, err := pgxpool.New(ctx, "postgres://user:pass@localhost:5432/dbname")
if err != nil {
    log.Fatal(err)
}
defer pool.Close()
```

`pgxpool` is a connection pool — multiple goroutines can use it safely at the same time.
Always use the pool, not a single connection.

**Why a pointer to `pgxpool.Pool`:**
The pool manages shared mutable state — open connections, idle connections, waiting goroutines.
Copying it by value would produce a shallow copy of that state.
Two copies managing the same underlying connections would corrupt each other.
A pointer ensures all code shares the same pool instance.

**Why a pool instead of a single connection:**
A single `pgx.Conn` is not safe for concurrent use — only one goroutine can use it at a time.
The worker pool in week 3 runs multiple goroutines simultaneously, all needing database access.
`pgxpool` manages a set of connections and hands them out as needed.

---

### 1.5 Writing queries with pgx

```go
// insert
_, err := pool.Exec(ctx,
    "INSERT INTO events (id, type, payload, status, retry_count) VALUES ($1, $2, $3, $4, $5)",
    event.ID, event.Type, event.Payload, event.Status, event.RetryCount,
)

// query rows
rows, err := pool.Query(ctx,
    "SELECT id, type, payload, status, retry_count FROM events WHERE status = $1",
    status,
)
defer rows.Close()

for rows.Next() {
    var e domain.NotificationEvent
    if err := rows.Scan(&e.ID, &e.Type, &e.Payload, &e.Status, &e.RetryCount); err != nil {
        return nil, err
    }
}

// always check rows.Err() after the loop
if err := rows.Err(); err != nil {
    return nil, err
}
```

`$1`, `$2` are PostgreSQL placeholders — not `?` like MySQL.
`Scan` maps columns to struct fields manually. No reflection tags, no magic.
`rows.Err()` catches network errors that stop iteration early — not just "no more rows".

---

### 1.6 Constructor pattern — unexported fields

Struct fields in Go can be unexported (lowercase). Callers outside the package cannot set them directly.
A constructor function is the only way to create the struct from outside the package:

```go
// Without constructor — does NOT compile from another package
repo := repository.PostgresEventRepository{
    pool: pool,  // ERROR: cannot refer to unexported field 'pool'
}

// With constructor — works from any package
func NewPostgresEventRepository(pool *pgxpool.Pool) *PostgresEventRepository {
    return &PostgresEventRepository{pool: pool}
}

repo := repository.NewPostgresEventRepository(pool)
```

Java equivalent:
```java
public class PostgresEventRepository {
    private final DataSource dataSource; // private field

    public PostgresEventRepository(DataSource dataSource) { // constructor
        this.dataSource = dataSource;
    }
}
```

Same concept — different syntax, no framework.

---

## 2. My Learning Summary

**What I learned:**
- Go uses raw SQL with a driver — no ORM, no generated queries, full visibility
- `pgxpool.Pool` is a connection pool safe for concurrent goroutine access
- Repository interface lives in `domain` (consumer), implementation in `repository` (producer)
- `defer rows.Close()` — always, or the connection is never returned to the pool
- `rows.Err()` — always check after the loop to catch network errors
- Unexported fields + constructor = Go's equivalent of private field + constructor in Java

**Key Go concepts:**
- `pgxpool.New(ctx, connString)` — creates a connection pool
- `pool.Exec(ctx, sql, args...)` — execute a statement, no rows returned
- `pool.Query(ctx, sql, args...)` — execute a query, returns rows
- `rows.Scan(&field1, &field2, ...)` — map columns to variables manually
- `rows.Close()` — must be deferred, returns connection to pool
- `rows.Err()` — check after loop for iteration errors
- `var _ domain.EventRepository = (*PostgresEventRepository)(nil)` — compile-time interface check

**What confused me at first:**
- Why `pgxpool.Pool` is passed as a pointer.
  The pool manages shared mutable state — copying it by value would corrupt connections.
  Always pass by pointer when the type manages shared state.

**What finally made it clear:**
- The unexported field + constructor pattern.
  Lowercase field = private. Constructor = the only door into the struct from outside the package.
  Same concept as Java private fields, different syntax.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Forgetting `defer rows.Close()` | Connection never returned to pool — pool exhausts under load |
| Not checking `rows.Err()` after loop | Network errors silently return partial results |
| Using `$1` style with MySQL | MySQL uses `?` — PostgreSQL uses `$1`, `$2` |
| Exporting struct fields to avoid a constructor | Exposes implementation details — use unexported fields + constructor |
| Using a single connection instead of a pool | Not safe for concurrent goroutine access |

---

## 3. Code Demo

### `internal/domain/repository.go` — interface in domain

```go
package domain

import "context"

type EventRepository interface {
    Insert(ctx context.Context, event NotificationEvent) error
    FindByStatus(ctx context.Context, status EventStatus) ([]NotificationEvent, error)
    UpdateStatus(ctx context.Context, id string, status EventStatus, retryCount int) error
}
```

### `internal/repository/postgres_event_repository.go`

```go
package repository

import (
    "context"
    "fmt"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/kadriyebarlak/notification-dispatcher/internal/domain"
)

type PostgresEventRepository struct {
    pool *pgxpool.Pool
}

func NewPostgresEventRepository(pool *pgxpool.Pool) *PostgresEventRepository {
    return &PostgresEventRepository{pool: pool}
}

func (r *PostgresEventRepository) Insert(ctx context.Context, event domain.NotificationEvent) error {
    event.ID = fmt.Sprintf("evt-%d", time.Now().UnixNano())
    _, err := r.pool.Exec(ctx,
        "INSERT INTO events (id, type, payload, status, retry_count) VALUES ($1, $2, $3, $4, $5)",
        event.ID, event.Type, event.Payload, event.Status, event.RetryCount,
    )
    return err
}

func (r *PostgresEventRepository) FindByStatus(ctx context.Context, status domain.EventStatus) ([]domain.NotificationEvent, error) {
    rows, err := r.pool.Query(ctx,
        "SELECT id, type, payload, status, retry_count FROM events WHERE status = $1",
        status,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var events []domain.NotificationEvent
    for rows.Next() {
        var e domain.NotificationEvent
        if err := rows.Scan(&e.ID, &e.Type, &e.Payload, &e.Status, &e.RetryCount); err != nil {
            return nil, err
        }
        events = append(events, e)
    }
    if err := rows.Err(); err != nil {
        return nil, err
    }
    return events, nil
}

func (r *PostgresEventRepository) UpdateStatus(ctx context.Context, id string, status domain.EventStatus, retryCount int) error {
    _, err := r.pool.Exec(ctx,
        "UPDATE events SET status=$1, retry_count=$2 WHERE id=$3",
        status, retryCount, id,
    )
    return err
}

var _ domain.EventRepository = (*PostgresEventRepository)(nil)
```

### Running PostgreSQL locally with Docker

```bash
docker run --name notification-db \
  -e POSTGRES_USER=notify \
  -e POSTGRES_PASSWORD=notify \
  -e POSTGRES_DB=notification_dispatcher \
  -p 5432:5432 \
  -d postgres:16
```

Stop: `docker stop notification-db`
Start again: `docker start notification-db`

---

## 4. Interview Takeaway

**Why no ORM in Go:**
Go encourages explicit code. Raw SQL with `pgx` means you always know exactly what query runs.
No N+1 surprises, no magic, no generated queries you don't understand.
The tradeoff is more code — but more control and visibility.

**Repository pattern in Go:**
Interface in `domain` (consumer), implementation in `repository` (producer).
The service depends on the interface — not the PostgreSQL implementation.
Swapping implementations (e.g. for an in-memory mock in tests) requires no changes to the service.

**Three rules for `pgx` row queries:**
1. Always `defer rows.Close()` — returns the connection to the pool
2. Always check `rows.Scan` errors inside the loop
3. Always check `rows.Err()` after the loop — catches network errors

**Constructor pattern:**
Unexported struct fields cannot be set from outside the package.
A constructor function (exported, capital letter) is the only door.
This is Go's equivalent of private fields with constructor injection.

---

## 5. Cleanup Notes

Day 11 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/domain/repository.go` — `EventRepository` interface
- `internal/repository/postgres_event_repository.go` — PostgreSQL implementation with constructor

**Note:** The repository is not wired into `main.go` yet.
That happens on Day 13 after migrations are set up on Day 12.