# Day 26 — Linting & Go Tooling

---

## 1. Original Lesson Explanation

### 1.1 Big picture

In Java you have Checkstyle, SpotBugs, SonarQube.
In Go the standard is `golangci-lint` — a fast, configurable linter that runs
multiple linters in parallel. Every serious Go project uses it.

Running lint before committing is not just style — it catches real bugs.
`go vet` catches things the compiler misses. `golangci-lint` goes further.

---

### 1.2 The three tools

**`gofmt`** — formats Go code. Non-negotiable.
Every Go developer runs it. VS Code runs it automatically on save.
No configuration — one standard format for all Go code.

```bash
gofmt -w .   # format all files in place
```

**`go vet`** — static analysis built into Go. Catches real bugs the compiler allows:
- Printf format string mismatches: `fmt.Sprintf("%d", "string")`
- Unreachable code
- Suspicious composite literals
- Incorrect mutex usage

```bash
go vet ./...
```

**`golangci-lint`** — runs many linters at once:

| Linter | What it catches |
|---|---|
| `errcheck` | Ignored errors — `json.Unmarshal(data, &v)` without checking err |
| `gosimple` | Simplifications — `x == true` instead of `x` |
| `ineffassign` | Assigned variable never used |
| `staticcheck` | Deprecated API usage, unreachable code, type issues |
| `unused` | Exported functions/types never used |
| `govet` | Same as `go vet` |

> **Türkçe özet:** `gofmt` kodu formatlar. `go vet` gerçek bugları yakalar.
> `golangci-lint` birden fazla linter'ı aynı anda çalıştırır.
> Üçü birlikte kodun hem doğru hem de temiz olmasını sağlar.

---

### 1.3 `.golangci.yml` configuration

```yaml
run:
  timeout: 5m

linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused

linters-settings:
  errcheck:
    check-type-assertions: true

issues:
  exclude-rules:
    - path: _test.go
      linters:
        - errcheck
```

`exclude-rules` skips `errcheck` in test files —
test setup code often ignores errors intentionally.

---

### 1.4 Silencing specific warnings

Sometimes ignoring an error is intentional.
Two options to silence `errcheck` for a specific line:

**Option A — blank identifier with comment:**
```go
_ = rows.Close() // error intentionally ignored — close error rarely meaningful
```

**Option B — `//nolint` directive:**
```go
rows.Close() //nolint:errcheck // close error rarely meaningful in read-only query
```

**Always add a comment explaining why.**
A `//nolint` without a reason is a red flag in code review —
it looks like someone silenced a warning without understanding it.

**Rule:** silence a linter warning only when you have a genuine reason.
Never silence to make the build pass without understanding why the linter flagged it.

---

### 1.5 What the lint run found

`errcheck` flagged three ignored `UpdateStatus` calls in `Dispatcher.Process`.

**Before:**
```go
d.repo.UpdateStatus(ctx, event.ID, domain.StatusDead, event.RetryCount)
```

**After:**
```go
if err := d.repo.UpdateStatus(ctx, event.ID, domain.StatusDead, event.RetryCount); err != nil {
    log.Printf("dispatcher: failed to update status to dead for event %s: %v", event.ID, err)
}
```

Log the error but continue — you cannot do much if a status update fails mid-dispatch.
This is the correct behaviour for a background worker.

One subtle detail in the max-retries path:
```go
if err := d.repo.UpdateStatus(ctx, event.ID, domain.StatusDead, event.RetryCount+1); err != nil {
    log.Printf("dispatcher: failed to update status to dead for event %s: %v", event.ID, err)
    return  // ← exit before the "marked as dead" log
}
log.Printf("dispatcher: event %s marked as dead after %d attempts", event.ID, d.maxRetries)
```

The `return` inside the error block is correct — if the status update failed,
the event is NOT dead yet. Do not log "marked as dead" until the update succeeds.

---

## 2. My Learning Summary

**What I learned:**
- `gofmt` — one standard format, no configuration, non-negotiable in Go
- `go vet` — catches real bugs the compiler allows
- `golangci-lint` — multiple linters in parallel, configurable per project
- `errcheck` catches ignored errors — one of the most valuable linters
- `//nolint` with a comment — correct way to silence a specific warning
- Always fix lint errors properly — understand why before silencing
- Log-and-continue pattern for background worker status update failures

**Key Go tooling:**
- `gofmt -w .` — format in place
- `go vet ./...` — static analysis
- `golangci-lint run ./...` — all configured linters
- `.golangci.yml` — linter configuration, committed to repo
- `//nolint:lintername` — silence specific linter on specific line
- `_ = expr` — blank identifier, explicitly ignoring a value

**What lint found in this project:**
- Three ignored `UpdateStatus` errors in `Dispatcher.Process`
- Fixed by wrapping in `if err :=` and logging — not silencing

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| `//nolint` without a comment | Looks like avoidance, not intentional decision |
| Disabling entire linters to fix build | Throws away real bug detection |
| Running lint only at CI | Catch issues locally before committing |
| Ignoring `errcheck` everywhere | Silent failures in production — real bugs |
| Not committing `.golangci.yml` | Other developers get different lint results |

---

## 3. Code Demo

### `.golangci.yml`

```yaml
run:
  timeout: 5m

linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused

linters-settings:
  errcheck:
    check-type-assertions: true

issues:
  exclude-rules:
    - path: _test.go
      linters:
        - errcheck
```

### `Makefile` targets

```makefile
lint:
	golangci-lint run ./...

vet:
	go vet ./...

fmt:
	gofmt -w .
```

### Fixed `Dispatcher.Process` — all errors handled

```go
func (d *Dispatcher) Process(ctx context.Context, event domain.NotificationEvent) {
    notifier, ok := d.registry.Get(event.Type)
    if !ok {
        log.Printf("dispatcher: no notifier for type %s", event.Type)
        if err := d.repo.UpdateStatus(ctx, event.ID, domain.StatusDead, event.RetryCount); err != nil {
            log.Printf("dispatcher: failed to update status to dead for event %s: %v", event.ID, err)
        }
        return
    }

    if err := notifier.Send(ctx, event); err != nil {
        log.Printf("dispatcher: failed to send %s (attempt %d): %v", event.ID, event.RetryCount+1, err)

        if event.RetryCount+1 >= d.maxRetries {
            if err := d.repo.UpdateStatus(ctx, event.ID, domain.StatusDead, event.RetryCount+1); err != nil {
                log.Printf("dispatcher: failed to update status to dead for event %s: %v", event.ID, err)
                return
            }
            log.Printf("dispatcher: event %s marked as dead after %d attempts", event.ID, d.maxRetries)
            return
        }

        if err := d.repo.UpdateStatus(ctx, event.ID, domain.StatusFailed, event.RetryCount+1); err != nil {
            log.Printf("dispatcher: failed to update status to failed for event %s: %v", event.ID, err)
        }
        return
    }

    if err := d.repo.UpdateStatus(ctx, event.ID, domain.StatusDelivered, event.RetryCount); err != nil {
        log.Printf("dispatcher: failed to update status to delivered for event %s: %v", event.ID, err)
    }
}
```

### Silencing intentional error ignores

```go
// Option A — blank identifier
_ = rows.Close() // close error rarely meaningful in read-only query

// Option B — nolint directive
rows.Close() //nolint:errcheck // close error rarely meaningful in read-only query
```

---

## 4. Interview Takeaway

**Why linting matters beyond style:**
`errcheck` catches ignored errors — a common source of silent production failures in Go.
`go vet` catches printf mismatches, incorrect mutex usage, and unreachable code.
These are real bugs, not style preferences. Linting is quality control.

**`golangci-lint` vs individual tools:**
Runs many linters in parallel — faster than running each separately.
Configurable via `.golangci.yml` — committed to the repo so the whole team gets the same results.
Industry standard — most Go teams use it in CI.

**How to handle intentional error ignores:**
Use `_ = expr` or `//nolint:errcheck` — both are explicit signals that the ignore is intentional.
Always add a comment explaining why. Without a comment, it looks like a mistake.

**The fix pattern for background workers:**
When a status update fails in a background worker, log the error and continue.
You cannot do much else — the worker should not crash or block other events.
If the update failed, the event stays in its current status and will be retried.

---

## 5. Cleanup Notes

Day 26 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `.golangci.yml` — lint configuration, committed to repo
- `internal/dispatcher/dispatcher.go` — fixed with proper error handling
- `Makefile` — updated with `lint`, `vet`, `fmt` targets