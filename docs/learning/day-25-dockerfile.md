# Day 25 — Dockerfile & Multi-Stage Build

---

## 1. Original Lesson Explanation

### 1.1 Big picture

A typical Spring Boot Docker image is 200–400MB.
It contains the JVM, all Spring dependencies, and the application JAR.
Every container pull, every deployment, every layer cache miss — hundreds of megabytes.

Go compiles to a single static binary. No JVM, no runtime dependencies.
The final Docker image for this project is **34MB**.
This is one of Go's most concrete production advantages.

---

### 1.2 Why multi-stage build

You need Go installed to compile the binary.
You do not need Go installed to run it.

A single-stage Dockerfile would include the entire Go toolchain — ~300MB of unnecessary weight.

Multi-stage build solves this:

```
Stage 1 (builder): Go installed → compile binary
Stage 2 (runtime): no Go needed → copy binary, run it
```

The final image only contains Stage 2. Stage 1 is discarded after the build.

> **Türkçe özet:** Go binary'yi derlemek için Go toolchain'e ihtiyacın var ama çalıştırmak için yok.
> Multi-stage build: ilk aşamada Go ile derle, ikinci aşamada sadece binary'yi kopyala.
> Final image küçük ve güvenli — Go toolchain yok, gereksiz paket yok.

---

### 1.3 Base image choices for the runtime stage

| Image | Size | Shell | SSL certs | Best for |
|---|---|---|---|---|
| `scratch` | ~0MB | No | No | Smallest possible, hardest to debug |
| `gcr.io/distroless/static` | ~2MB | No | Yes | Production Go services |
| `alpine` | ~5MB | Yes | Yes | Development, easy to debug |

This project uses `alpine` — small, debuggable, and familiar.

---

### 1.4 Docker layer caching — why copy order matters

```dockerfile
# Layer 1 — only changes when dependencies change
COPY go.mod go.sum ./
RUN go mod download

# Layer 2 — changes every time source code changes
COPY . .
RUN go build ...
```

Docker caches each layer. If source code changes but `go.mod` and `go.sum` do not,
Docker skips `go mod download` — saving 30–60 seconds per build.

If everything was copied at once:
```dockerfile
COPY . .          # invalidated on every source change
RUN go mod download  # runs every time — slow
```

**Rule: copy what changes least first, what changes most last.**

`./` and `.` in `COPY` mean the same thing — both refer to the current `WORKDIR`.
The difference is style only. Docker accepts both.

---

### 1.5 Key build flags

**`CGO_ENABLED=0`** — disables C bindings.
Produces a fully static binary that runs on any Linux without libc.
Required for scratch/distroless images. Good practice everywhere.

If a package depends on CGO and you build with `CGO_ENABLED=0`,
the **build fails at compile time** — not at runtime.
The compiler refuses to link C dependencies immediately.
This is better than a runtime failure — you find out right away.

**`GOOS=linux`** — compiles for Linux even when building on Mac.
Without this, building on Mac produces a Darwin binary that will not run in a Linux container.

---

### 1.6 `.dockerignore`

Prevents unnecessary files from being sent to the Docker build context:

```
.git
.env
*.md
docs/
```

Smaller build context = faster builds. Sensitive files like `.env` never enter the build.

---

### 1.7 Image size comparison

| Technology | Typical image size |
|---|---|
| Spring Boot + OpenJDK | 200–400MB |
| This Go service | **34MB** |

Same category of service — REST API, database, async workers.
~10x smaller. Faster pulls, less storage, faster Kubernetes scheduling.

---

## 2. My Learning Summary

**What I learned:**
- Multi-stage builds separate compilation from runtime — only the binary goes to production
- Layer ordering is deliberate — `go.mod`/`go.sum` first to maximise cache hits
- `CGO_ENABLED=0` — static binary, no libc, runs on any Linux
- `GOOS=linux` — cross-compilation from Mac to Linux
- `./` and `.` in COPY are the same — just style difference
- CGO build failures are compile-time, not runtime — better error discovery
- Final image: 34MB vs 200–400MB for Spring Boot — concrete, measurable advantage

**Key concepts:**
- `FROM golang:1.24-alpine AS builder` — named build stage
- `COPY --from=builder /app/server .` — copy artefact from previous stage
- `CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server` — static Linux binary
- `EXPOSE 8080` — documents the port, does not publish it
- `CMD ["./server"]` — default command when container starts
- `.dockerignore` — excludes files from build context

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Single-stage Dockerfile | Final image includes entire Go toolchain — ~300MB |
| Copying all files before `go mod download` | Cache invalidated on every source change — slow builds |
| Missing `CGO_ENABLED=0` | Binary may depend on libc — fails in scratch/distroless |
| Missing `GOOS=linux` on Mac | Darwin binary — does not run in Linux container |
| Not using `.dockerignore` | `.env` credentials may enter the build context |

---

## 3. Code Demo

### `Dockerfile`

```dockerfile
# Stage 1 — build
FROM golang:1.24-alpine AS builder

WORKDIR /app

# copy dependency files first — Docker layer cache
COPY go.mod go.sum ./
RUN go mod download

# copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

# Stage 2 — runtime
FROM alpine:3.20

WORKDIR /app

# copy binary from builder
COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]
```

### `.dockerignore`

```
.git
.env
*.md
docs/
```

### `Makefile` targets

```makefile
docker-build:
	docker build -t notification-dispatcher .

docker-run:
	docker run --rm \
		-e DATABASE_URL=postgres://notify:notify@host.docker.internal:5432/notification_dispatcher?sslmode=disable \
		-p 8080:8080 \
		notification-dispatcher
```

### Image size result

```bash
docker images notification-dispatcher
REPOSITORY                TAG       IMAGE ID       CREATED       SIZE
notification-dispatcher   latest    a3f1ba0268d3   14 min ago    34.3MB
```

---

## 4. Interview Takeaway

**Why multi-stage builds:**
Compilation requires Go toolchain (~300MB). Running the binary does not.
Multi-stage builds keep the toolchain out of the final image.
Result: a small, secure, production-ready image with only what is needed to run.

**Go vs Java image size:**
Go compiles to a single static binary — no JVM, no runtime dependencies.
This project's image: 34MB. Equivalent Spring Boot image: 200–400MB.
~10x smaller means faster pulls, less storage, faster Kubernetes pod scheduling.

**`CGO_ENABLED=0` explained:**
Disables C bindings — produces a fully static binary.
Required for scratch and distroless images. Good practice for all Go services.
CGO build failures are compile-time — you find out immediately, not at runtime.

**Docker layer caching strategy:**
Copy files that change rarely first — dependency files (`go.mod`, `go.sum`).
Copy files that change often last — source code.
Docker caches each layer. A cache hit on `go mod download` saves 30–60 seconds per build.

---

## 5. Cleanup Notes

Day 25 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `Dockerfile` — production multi-stage build
- `.dockerignore` — build context exclusions
- `Makefile` — updated with `docker-build` and `docker-run` targets