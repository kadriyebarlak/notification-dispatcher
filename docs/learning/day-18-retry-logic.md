# Day 18 — Retry Logic & Failure Handling

---

## 1. Original Lesson Explanation

### 1.1 Big picture

After Day 17, when a notifier fails the event is immediately set to `FAILED` and never
tried again. That is too strict for a real notification service.
Networks fail, external services have brief outages.
A good system retries a few times before giving up.

`retry_count` already exists in the domain and database. Day 18 uses it.

---

### 1.2 The retry flow

```
Notifier fails
     ↓
retry_count+1 < maxRetries?
     ├── YES → status = FAILED, retry_count++
     │         (dispatcher picks it up again on next tick)
     └── NO  → status = DEAD
               (never retried again)
```

The dispatcher queries for both `PENDING` and `FAILED` events.
Once `retry_count` reaches `maxRetries`, status becomes `DEAD` —
and `DEAD` is never included in the query.

---

### 1.3 Why `retry_count+1` and not `retry_count`

When `Process` runs, `retry_count` holds the value from the **previous** failure.
The `+1` represents the attempt **currently happening** — before incrementing.

| Attempt | `retry_count` when Process runs | Check | Result |
|---|---|---|---|
| 1st | 0 | `0+1=1 >= 3`? No | FAILED, retry_count=1 |
| 2nd | 1 | `1+1=2 >= 3`? No | FAILED, retry_count=2 |
| 3rd | 2 | `2+1=3 >= 3`? Yes | DEAD, retry_count=3 |

Without the `+1`: `retry_count >= maxRetries` would allow a 4th attempt before
setting DEAD — one extra retry that should not happen.

---

### 1.4 Retry logic in `Process`

```go
if err := notifier.Send(ctx, event); err != nil {
    log.Printf("dispatcher: failed to send %s (attempt %d): %v",
        event.ID, event.RetryCount+1, err)

    if event.RetryCount+1 >= d.maxRetries {
        d.repo.UpdateStatus(ctx, event.ID, domain.StatusDead, event.RetryCount+1)
        log.Printf("dispatcher: event %s marked as dead after %d attempts",
            event.ID, d.maxRetries)
        return
    }

    d.repo.UpdateStatus(ctx, event.ID, domain.StatusFailed, event.RetryCount+1)
    return
}
```

Use `d.maxRetries` — the struct field — not a package-level const.
This makes `maxRetries` configurable per dispatcher instance.

> **Türkçe özet:** Notifier başarısız olursa retry_count artar.
> maxRetries'e ulaşıldıysa status DEAD olur — bir daha denenmez.
> Ulaşılmadıysa FAILED olur — dispatcher bir sonraki tick'te tekrar alır.

---

### 1.5 `FindByStatuses` — variadic method

The dispatcher now queries for both `PENDING` and `FAILED`:

```go
events, err := d.repo.FindByStatuses(ctx, domain.StatusPending, domain.StatusFailed)
```

Variadic parameter in the interface:
```go
FindByStatuses(ctx context.Context, statuses ...domain.EventStatus) ([]domain.NotificationEvent, error)
```

PostgreSQL implementation — convert to `[]string` for `pgx`, use `ANY($1::text[])`:

```go
func (r *PostgresEventRepository) FindByStatuses(ctx context.Context, statuses ...domain.EventStatus) ([]domain.NotificationEvent, error) {
    statusValues := make([]string, 0, len(statuses))
    for _, status := range statuses {
        statusValues = append(statusValues, string(status))
    }

    rows, err := r.pool.Query(ctx,
        "SELECT id, type, payload, status, retry_count FROM events WHERE status = ANY($1::text[])",
        statusValues,
    )
    // ... scan rows, check rows.Err()
}
```

---

### 1.6 Exponential backoff — MVP vs production

**MVP approach (this project):**
The polling interval provides natural delay between retries.
No additional backoff logic needed. Keep it simple.

**Production approach:**
Store `updated_at` in the events table.
Only retry events where `updated_at` is old enough:
```sql
WHERE status = 'failed'
AND updated_at < NOW() - INTERVAL '30 seconds' * retry_count
```

This gives true exponential backoff without sleeping in Go code.
Mention this in interviews — shows you understand the production tradeoff.

---

## 2. My Learning Summary

**What I learned:**
- `retry_count+1 >= maxRetries` — the `+1` accounts for the current attempt
- `maxRetries` belongs in the struct field, not a package-level const — configurable
- Dispatcher queries `PENDING` and `FAILED` — `DEAD` is never retried
- `FindByStatuses` with variadic `...EventStatus` — flexible, clean interface
- `ANY($1::text[])` — PostgreSQL syntax for matching against a list of values
- MVP retry uses polling interval as natural delay — production uses `updated_at` comparison

**Key Go concepts:**
- Variadic parameters: `func f(args ...T)` — accepts one or more values of type T
- `make([]string, 0, len(statuses))` — pre-allocate slice with known capacity
- Package const vs struct field — prefer struct field when value should be configurable
- `d.maxRetries` not `maxRetries` — use the instance field, not the global constant

**What confused me at first:**
- Having both a `maxRetries` struct field and a package-level `const maxRetries`.
  `Process` was using the const — making the struct field unused and inconsistent.
  Fix: remove the const, use `d.maxRetries` everywhere.

**What finally made it clear:**
- Drawing out the retry attempts on paper:
  Attempt 1: retry_count=0, Attempt 2: retry_count=1, Attempt 3: retry_count=2.
  `retry_count` is the value from the last failure — `+1` is the current attempt.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| `retry_count >= maxRetries` without `+1` | Allows one extra retry — 4 attempts instead of 3 |
| Package-level const instead of struct field | maxRetries cannot be configured per instance |
| Not querying FAILED events in dispatcher | Failed events are never retried |
| Querying DEAD events in dispatcher | Dead events get retried — defeats the purpose |
| Not converting `EventStatus` to `string` for pgx | pgx cannot map custom named types to SQL arrays |

---

## 3. Code Demo

### Updated `internal/dispatcher/dispatcher.go`

```go
type Dispatcher struct {
    repo       domain.EventRepository
    pool       *worker.WorkerPool
    registry   *notifier.NotifierRegistry
    interval   time.Duration
    maxRetries int  // configurable — not a package constant
}

func (d *Dispatcher) dispatch(ctx context.Context) {
    events, err := d.repo.FindByStatuses(ctx, domain.StatusPending, domain.StatusFailed)
    if err != nil {
        log.Printf("dispatcher: failed to fetch events: %v", err)
        return
    }

    for _, event := range events {
        if err := d.repo.UpdateStatus(ctx, event.ID, domain.StatusProcessing, event.RetryCount); err != nil {
            log.Printf("dispatcher: failed to update status: %v", err)
            continue
        }
        d.pool.Submit(event)
    }
}

func (d *Dispatcher) Process(ctx context.Context, event domain.NotificationEvent) {
    notifier, ok := d.registry.Get(event.Type)
    if !ok {
        log.Printf("dispatcher: no notifier for type %s", event.Type)
        d.repo.UpdateStatus(ctx, event.ID, domain.StatusDead, event.RetryCount)
        return
    }

    if err := notifier.Send(ctx, event); err != nil {
        log.Printf("dispatcher: failed to send %s (attempt %d): %v",
            event.ID, event.RetryCount+1, err)

        if event.RetryCount+1 >= d.maxRetries {
            d.repo.UpdateStatus(ctx, event.ID, domain.StatusDead, event.RetryCount+1)
            log.Printf("dispatcher: event %s marked as dead after %d attempts",
                event.ID, d.maxRetries)
            return
        }

        d.repo.UpdateStatus(ctx, event.ID, domain.StatusFailed, event.RetryCount+1)
        return
    }

    d.repo.UpdateStatus(ctx, event.ID, domain.StatusDelivered, event.RetryCount)
}
```

### Updated `internal/domain/repository.go` interface

```go
type EventRepository interface {
    Insert(ctx context.Context, event NotificationEvent) error
    FindByStatus(ctx context.Context, status EventStatus) ([]NotificationEvent, error)
    FindByStatuses(ctx context.Context, statuses ...EventStatus) ([]NotificationEvent, error)
    UpdateStatus(ctx context.Context, id string, status EventStatus, retryCount int) error
}
```

### `FindByStatuses` in `postgres_event_repository.go`

```go
func (r *PostgresEventRepository) FindByStatuses(ctx context.Context, statuses ...domain.EventStatus) ([]domain.NotificationEvent, error) {
    statusValues := make([]string, 0, len(statuses))
    for _, status := range statuses {
        statusValues = append(statusValues, string(status))
    }

    rows, err := r.pool.Query(ctx,
        "SELECT id, type, payload, status, retry_count FROM events WHERE status = ANY($1::text[])",
        statusValues,
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
```

### Testing retry behaviour

```go
// temporarily in main.go
&notifier.FakeEmailNotifier{ShouldFail: true}
```

```bash
# submit event, watch logs across 3 ticks, check final status
docker exec -it notification-db psql -U notify \
  -d notification_dispatcher \
  -c "SELECT id, status, retry_count FROM events;"

# expected after 3 failed attempts:
# status = dead, retry_count = 3
```

---

## 4. Interview Takeaway

**How retry logic works in this service:**
Failed events are re-queried on each dispatcher tick (`PENDING` and `FAILED`).
`retry_count` tracks how many attempts have been made.
After `maxRetries` attempts, status becomes `DEAD` — never retried again.
`maxRetries` is a constructor parameter — configurable per instance.

**Why `retry_count+1` in the check:**
`retry_count` holds the value from the previous failure.
`+1` represents the attempt currently in progress.
Without `+1`, the check fires one attempt too late — one extra unwanted retry.

**MVP vs production backoff:**
MVP: polling interval provides natural delay — simple and sufficient for one instance.
Production: `updated_at` column + SQL interval check gives true exponential backoff
across distributed instances without any sleeping in Go code.

**`SELECT FOR UPDATE SKIP LOCKED` revisited:**
For multi-instance deployments, combine retry logic with `SELECT FOR UPDATE SKIP LOCKED`
to prevent two instances from retrying the same failed event simultaneously.

---

## 5. Cleanup Notes

Day 18 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/dispatcher/dispatcher.go` — updated with retry logic and `d.maxRetries`
- `internal/domain/repository.go` — `FindByStatuses` added to interface
- `internal/repository/postgres_event_repository.go` — `FindByStatuses` implemented

**Remember:** remove `ShouldFail: true` from `main.go` after testing.
The production registry should use `ShouldFail: false` (the default zero value).