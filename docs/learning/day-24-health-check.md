# Day 24 — Health Check & Readiness Endpoints

---

## 1. Original Lesson Explanation

### 1.1 Big picture

Every production service needs two endpoints that tell the infrastructure
whether the service is alive and ready to serve traffic.

In Spring Boot these come free with Spring Actuator — `GET /actuator/health`.
In Go you write them yourself. They are small but important.

---

### 1.2 Liveness vs readiness — why two endpoints

**`/health` — liveness probe**
Is the process alive? Can it respond to HTTP?
If this fails, Kubernetes restarts the container.
Very simple — no external dependencies checked.

```json
{"status": "ok"}
```

**`/ready` — readiness probe**
Is the service ready to handle traffic? Are its dependencies healthy?
If this fails, Kubernetes stops sending traffic — but does not restart.
Check the database connection here.

```json
{"status": "ok"}                                          // DB healthy
{"status": "unavailable", "reason": "database unreachable"} // DB down — 503
```

> **Türkçe özet:** `/health` sadece "process yaşıyor mu?" diye sorar — basit, hızlı.
> `/ready` "servis traffic almaya hazır mı?" diye sorar — DB bağlantısını kontrol eder.
> Kubernetes bu ikisini farklı amaçlarla kullanır.

---

### 1.3 Why `/health` returning 200 with DB down is correct

Liveness and readiness have different jobs:

**Liveness** answers: "should Kubernetes restart this container?"
If the process is alive and responding — no. Restarting the app does not fix a database outage.
Restarting would cause unnecessary downtime on top of an existing problem.

**Readiness** answers: "should Kubernetes send traffic to this instance?"
If the DB is down — no. `/ready` returns 503. Traffic is rerouted to healthy instances.
The container stays running, waiting for the DB to recover.

One endpoint that checks the DB for both liveness and readiness would cause Kubernetes
to restart healthy containers during a database outage — making a bad situation worse.

---

### 1.4 Why this matters in production

Without health endpoints:
- Kubernetes cannot tell if a container is broken — traffic keeps going to dead instances
- Rolling deployments cannot wait for the service to be ready before removing old instances
- A failing DB connection pool is invisible to the load balancer

With health endpoints:
- `/ready` returning 503 reroutes traffic away from the affected instance automatically
- Rolling deployments wait for `/ready` to return 200 before cutting over
- `/health` proves the HTTP server itself is responding

---

### 1.5 Implementation

**`/health` — stateless, no dependencies:**

```go
func HealthHandler(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

**`/ready` — checks database connectivity:**

```go
type ReadinessHandler struct {
    db *pgxpool.Pool
}

func NewReadinessHandler(db *pgxpool.Pool) *ReadinessHandler {
    return &ReadinessHandler{db: db}
}

func (h *ReadinessHandler) Ready(w http.ResponseWriter, r *http.Request) {
    if err := h.db.Ping(r.Context()); err != nil {
        writeJSON(w, http.StatusServiceUnavailable, map[string]string{
            "status": "unavailable",
            "reason": "database unreachable",
        })
        return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

`r.Context()` is passed to `Ping` — if the request times out, the ping is cancelled too.

---

### 1.6 Wiring in `main.go`

```go
r.Get("/health", handler.HealthHandler)
r.Get("/ready", handler.NewReadinessHandler(pool).Ready)
```

---

### 1.7 Docker Compose healthcheck

```yaml
healthcheck:
  test: ["CMD-SHELL", "curl -f http://localhost:8080/health || exit 1"]
  interval: 10s
  timeout: 5s
  retries: 3
```

---

## 2. My Learning Summary

**What I learned:**
- Liveness and readiness serve different purposes — never combine them into one endpoint
- `/health` — process alive, no external checks, very fast
- `/ready` — dependencies healthy, returns 503 if DB is down
- Returning 200 from `/health` with DB down is correct — restarting the app does not fix the DB
- `r.Context()` passed to `Ping` — request timeout propagates into the DB health check
- `ReadinessHandler` is a struct — needs `*pgxpool.Pool` dependency
- `HealthHandler` is a plain function — no dependencies, no struct needed

**Key Go concepts:**
- `http.StatusServiceUnavailable` — 503, standard for "dependency unavailable"
- `pool.Ping(ctx)` — lightweight DB connectivity check
- Plain function handler vs struct handler — use struct only when you have dependencies
- `r.Context()` in readiness check — honours request timeout set by middleware

**What would happen without these endpoints:**
- No liveness: Kubernetes cannot detect a frozen or deadlocked process
- No readiness: traffic is sent to instances with broken DB connections
- No separation: DB outage triggers container restarts — worsens the situation

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Checking DB in liveness probe | DB outage triggers container restarts — wrong response |
| Not passing `r.Context()` to Ping | Request timeout is ignored — ping may hang |
| Using a struct for HealthHandler | No dependencies needed — plain function is cleaner |
| Returning 200 from `/ready` when DB is down | Traffic sent to broken instance — requests fail |
| Not adding healthcheck to docker-compose | Container marked healthy before service is actually ready |

---

## 3. Code Demo

### `internal/handler/health.go`

```go
package handler

import (
    "net/http"

    "github.com/jackc/pgx/v5/pgxpool"
)

func HealthHandler(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type ReadinessHandler struct {
    db *pgxpool.Pool
}

func NewReadinessHandler(db *pgxpool.Pool) *ReadinessHandler {
    return &ReadinessHandler{db: db}
}

func (h *ReadinessHandler) Ready(w http.ResponseWriter, r *http.Request) {
    if err := h.db.Ping(r.Context()); err != nil {
        writeJSON(w, http.StatusServiceUnavailable, map[string]string{
            "status": "unavailable",
            "reason": "database unreachable",
        })
        return
    }
    writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

### `cmd/server/main.go` — route registration

```go
r.Get("/health", handler.HealthHandler)
r.Get("/ready", handler.NewReadinessHandler(pool).Ready)
```

### Manual testing

```bash
# liveness — always 200 if process is running
curl -s http://localhost:8080/health | jq
# {"status": "ok"}

# readiness — 200 when DB is up
curl -s http://localhost:8080/ready | jq
# {"status": "ok"}

# readiness — 503 when DB is down
docker-compose stop db
curl -s http://localhost:8080/ready | jq
# {"status": "unavailable", "reason": "database unreachable"}

docker-compose start db
```

### Updated `docker-compose.yml` healthcheck

```yaml
healthcheck:
  test: ["CMD-SHELL", "curl -f http://localhost:8080/health || exit 1"]
  interval: 10s
  timeout: 5s
  retries: 3
```

---

## 4. Interview Takeaway

**Why two health endpoints:**
Liveness and readiness answer different questions.
Liveness: "should Kubernetes restart this container?" — process alive check only.
Readiness: "should Kubernetes send traffic here?" — dependency checks included.
Mixing them causes container restarts during database outages — the wrong response.

**Why `/health` returns 200 with DB down:**
The process is alive. Restarting it would not fix a database problem.
Kubernetes should keep the container running and wait for the DB to recover.
`/ready` handles the traffic routing — it returns 503 so traffic goes elsewhere.

**Standard HTTP status codes:**
- `200 OK` — healthy and ready
- `503 Service Unavailable` — running but not ready to serve traffic

**Production considerations:**
- Pass `r.Context()` to `Ping` — honours the request timeout from middleware
- Keep `/health` extremely simple — if it fails, Kubernetes restarts the container
- Add both endpoints to Docker Compose and Kubernetes manifests

---

## 5. Cleanup Notes

Day 24 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `internal/handler/health.go` — liveness and readiness handlers
- `cmd/server/main.go` — updated with health routes
- `docker-compose.yml` — updated with healthcheck