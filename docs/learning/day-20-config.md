# Day 20 — Configuration & Environment Variables

---

## 1. Original Lesson Explanation

### 1.1 Big picture

After Day 19, `main.go` has hardcoded values scattered through it:

```go
dbURL := "postgres://notify:notify@localhost:5432/notification_dispatcher?sslmode=disable"
workerPool := worker.NewWorkerPool(3)
disp := dispatcher.NewDispatcher(..., 30*time.Second, 3)
```

This is fine for development but wrong for production.
Different environments — local, staging, production — need different values.
You should never have to change code to change configuration.

This is the **12-factor app** principle: store configuration in the environment, not in the code.

---

### 1.2 Why it matters

In Spring Boot you have `application.properties` or `application.yml` —
the framework loads it automatically.
In Go there is no built-in equivalent. You load config yourself.

The idiomatic Go pattern is a **config struct** loaded once at startup:

```go
type Config struct {
    DatabaseURL        string
    Port               string
    WorkerCount        int
    MaxRetries         int
    DispatcherInterval time.Duration
}
```

---

### 1.3 Loading config from environment

```go
func LoadConfig() Config {
    return Config{
        DatabaseURL:        getEnv("DATABASE_URL", "postgres://..."),
        Port:               getEnv("PORT", ":8080"),
        WorkerCount:        getEnvInt("WORKER_COUNT", 3),
        MaxRetries:         getEnvInt("MAX_RETRIES", 3),
        DispatcherInterval: getEnvDuration("DISPATCHER_INTERVAL", 30*time.Second),
    }
}
```

Default values mean the service works locally without any environment setup.
In production, environment variables override the defaults.

> **Türkçe özet:** Spring'de `application.properties` var, framework okur.
> Go'da config struct tanımlarsın, env var'lardan kendin yüklersin.
> Default değerler sayesinde local'de hiçbir şey ayarlamadan çalışır.
> Production'da env var'lar default'ları ezer.

---

### 1.4 The risk of default values — fail fast

**The problem:**
If `DATABASE_URL` is not set in production, the app silently connects to
the default local development database instead of failing.
The service runs with the wrong database configuration — a dangerous silent failure.

**The production pattern — required values with no default:**

```go
func LoadConfig() (Config, error) {
    dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" {
        return Config{}, errors.New("DATABASE_URL is required")
    }

    return Config{
        DatabaseURL: dbURL,
        Port:        getEnv("PORT", ":8080"),
        // other optional values with defaults
    }, nil
}
```

In `main.go`:
```go
cfg, err := config.LoadConfig()
if err != nil {
    log.Fatal("config error:", err)
}
```

**Fail fast** — refuse to start if a critical value is missing.
Better to crash loudly at startup than silently connect to the wrong database.

For the MVP, keeping a default for `DATABASE_URL` is acceptable for local development.
In production, critical values should be required — no fallback.

---

### 1.5 `.env.example`

Committed to the repo — documents every configuration option:

```bash
DATABASE_URL=postgres://notify:notify@localhost:5432/notification_dispatcher?sslmode=disable
PORT=:8080
WORKER_COUNT=3
MAX_RETRIES=3
DISPATCHER_INTERVAL=30s
```

The actual `.env` file — with real credentials — is never committed. Add to `.gitignore`.

---

### 1.6 Port convention

`http.Server.Addr` expects `":8080"` — with the colon.
Store the full address in config including the colon:

```go
Port: getEnv("PORT", ":8080"),
```

Or store just the number and add the colon at the usage site:
```go
Addr: ":" + cfg.Port,
```

The second option is cleaner — config stores the port number, caller adds the colon.
Be consistent — pick one and stick to it.

---

## 2. My Learning Summary

**What I learned:**
- Config struct loaded from env vars is the idiomatic Go pattern — no framework needed
- Default values make local development easy but hide missing production config
- Critical values like `DATABASE_URL` should be required in production — fail fast
- `time.ParseDuration` accepts `"30s"`, `"1m"`, `"5m"` — flexible duration config
- `.env.example` is committed, `.env` is gitignored — standard practice
- Port should include the colon or be added consistently at the usage site

**Key Go concepts:**
- `os.Getenv(key)` — returns empty string if not set
- `strconv.Atoi(v)` — converts string to int, returns error if invalid
- `time.ParseDuration(v)` — parses `"30s"`, `"1m30s"`, `"1h"` etc.
- Fail fast pattern — validate required config at startup, refuse to run if missing
- Config struct as a single source of truth — passed through constructors

**What confused me at first:**
- Why not just use `os.Getenv` directly everywhere?
  Scattered `os.Getenv` calls make it hard to see all config at a glance,
  hard to test, and hard to add validation. A config struct centralises everything.

**What finally made it clear:**
- The fail fast argument. Silent defaults in production connect to the wrong database.
  Explicit required values crash loudly at startup — much easier to debug.

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Default `DATABASE_URL` in production | Silently connects to wrong DB — dangerous |
| Committing `.env` file | Credentials in version control — security risk |
| Not committing `.env.example` | Other developers don't know what variables are needed |
| `os.Getenv` scattered through the code | Hard to find all config options, hard to test |
| Port without colon in `http.Server.Addr` | Server fails to bind — silent startup error |

---

## 3. Code Demo

### `internal/config/config.go`

```go
package config

import (
    "os"
    "strconv"
    "time"
)

type Config struct {
    DatabaseURL        string
    Port               string
    WorkerCount        int
    MaxRetries         int
    DispatcherInterval time.Duration
}

func LoadConfig() Config {
    return Config{
        DatabaseURL:        getEnv("DATABASE_URL", "postgres://notify:notify@localhost:5432/notification_dispatcher?sslmode=disable"),
        Port:               getEnv("PORT", ":8080"),
        WorkerCount:        getEnvInt("WORKER_COUNT", 3),
        MaxRetries:         getEnvInt("MAX_RETRIES", 3),
        DispatcherInterval: getEnvDuration("DISPATCHER_INTERVAL", 30*time.Second),
    }
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

func getEnvInt(key string, fallback int) int {
    if v := os.Getenv(key); v != "" {
        if i, err := strconv.Atoi(v); err == nil {
            return i
        }
    }
    return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
    if v := os.Getenv(key); v != "" {
        if d, err := time.ParseDuration(v); err == nil {
            return d
        }
    }
    return fallback
}
```

### Production-grade `LoadConfig` with required values

```go
func LoadConfig() (Config, error) {
    dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" {
        return Config{}, errors.New("DATABASE_URL is required")
    }

    return Config{
        DatabaseURL:        dbURL,
        Port:               getEnv("PORT", ":8080"),
        WorkerCount:        getEnvInt("WORKER_COUNT", 3),
        MaxRetries:         getEnvInt("MAX_RETRIES", 3),
        DispatcherInterval: getEnvDuration("DISPATCHER_INTERVAL", 30*time.Second),
    }, nil
}
```

### `.env.example`

```bash
DATABASE_URL=postgres://notify:notify@localhost:5432/notification_dispatcher?sslmode=disable
PORT=:8080
WORKER_COUNT=3
MAX_RETRIES=3
DISPATCHER_INTERVAL=30s
```

### Testing config override

```bash
# default config
make run

# override port
PORT=:9090 make run
# server listens on :9090

# override dispatcher interval
DISPATCHER_INTERVAL=5s make run
# dispatcher polls every 5 seconds
```

---

## 4. Interview Takeaway

**12-factor app configuration:**
Store configuration in the environment — not in code.
Different environments (local, staging, production) use different env vars.
The code never changes — only the environment does.

**Config struct pattern in Go:**
Define a `Config` struct. Load it once at startup with `LoadConfig()`.
Pass the struct through constructors. Never call `os.Getenv` in business logic.
Centralised, testable, easy to audit.

**Fail fast for critical values:**
Optional config (port, worker count) — defaults are fine.
Critical config (database URL, API keys) — require explicitly, refuse to start if missing.
A service that crashes at startup with a clear error is far better than one that
silently connects to the wrong database for hours.

**`.env.example` pattern:**
`.env.example` is committed — documents all configuration options for the team.
`.env` is gitignored — never commit credentials to version control.

---

## 5. Cleanup Notes

Day 20 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/config/config.go` — config struct, used by main.go
- `.env.example` — committed, documents all env vars
- `.env` — gitignored, never committed

**Add to `.gitignore` if not already there:**
```
.env
```