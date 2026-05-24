# Day 12 — Database Migrations with goose

---

## 1. Original Lesson Explanation

### 1.1 Big picture

After Day 11, PostgreSQL is running but empty. There is no `events` table.
If you called `Insert`, the query would fail immediately.

In Spring Boot you use Flyway or Liquibase — they run SQL migration files automatically on startup.
Go has no built-in equivalent, but the community uses lightweight tools.
**`goose`** is the simplest option — clean CLI, works well embedded in a Go project,
and is widely used in production Go services.

---

### 1.2 Why migrations matter

You never create tables by hand in production. Tables created manually cannot be reproduced,
versioned, or rolled back. Migrations solve this:

- Every schema change is a versioned SQL file
- Migrations run in order — always the same result on any machine
- You can roll back if something goes wrong
- New team members run one command and have the exact same database

Same concept as Flyway — different tool, same principle.

> **Türkçe özet:** Tabloları elle oluşturmak production'da tehlikelidir — tekrar üretilemez,
> versiyon kontrolüne girmez. Migration dosyaları bu problemi çözer: her şema değişikliği
> sıralı SQL dosyası olarak saklanır, her ortamda aynı sonucu verir.

---

### 1.3 How goose works

Migration files live in a `migrations/` folder.
Each file has a version number prefix and contains `-- +goose Up` and `-- +goose Down` sections:

```sql
-- +goose Up
CREATE TABLE events (
    ...
);

-- +goose Down
DROP TABLE events;
```

`Up` runs when you migrate forward. `Down` runs when you roll back.

`goose` tracks which migrations have run in a `goose_db_version` table it manages automatically.
This is why you see two tables after running `migrate-up` — `events` and `goose_db_version`.

---

### 1.4 Two ways to use goose

**CLI tool** — run migrations from the terminal:
```bash
goose -dir migrations postgres "postgres://..." up
```

**Embedded in Go code** — run migrations on startup:
```go
import "github.com/pressly/goose/v3"

if err := goose.Up(db, "migrations"); err != nil {
    log.Fatal(err)
}
```

For this project, the CLI is used for day-to-day development via `make` targets.

---

### 1.5 Go version note

`goose` and some other tools require a recent Go version.
Always update Go before installing new tools to avoid version conflicts:

```bash
brew upgrade go      # macOS
go version           # verify
go mod tidy          # update go.mod
```

---

## 2. My Learning Summary

**What I learned:**
- Migration files are versioned SQL with `-- +goose Up` and `-- +goose Down` sections
- `goose` tracks migration history in a `goose_db_version` table automatically
- CLI usage is the standard approach for local development — `make` targets wrap the commands
- Same concept as Flyway/Liquibase — different tool, cleaner syntax
- Always update Go before installing new tools to avoid version conflicts

**Key concepts:**
- `-- +goose Up` — SQL that runs when migrating forward
- `-- +goose Down` — SQL that rolls back the migration
- `goose_db_version` — internal tracking table, do not modify manually
- `make migrate-up` — runs all pending migrations
- `make migrate-down` — rolls back the last migration
- `make migrate-status` — shows which migrations have run

**Common mistakes to remember:**

| Mistake | Why it is wrong |
|---|---|
| Creating tables manually in the DB | Cannot be reproduced, not version controlled |
| Forgetting the `-- +goose Down` section | Cannot roll back — always write both directions |
| Running migrations without `sslmode=disable` locally | Connection refused — local Docker has no SSL |
| Not running `go mod tidy` after updating Go | `go.mod` version mismatch causes build issues |

---

## 3. Code Demo

### `migrations/00001_create_events_table.sql`

```sql
-- +goose Up
CREATE TABLE events (
    id          VARCHAR(255) PRIMARY KEY,
    type        VARCHAR(100) NOT NULL,
    payload     TEXT         NOT NULL,
    status      VARCHAR(50)  NOT NULL DEFAULT 'pending',
    retry_count INT          NOT NULL DEFAULT 0,
    created_at  TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP    NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE events;
```

### `Makefile` — migration targets

```makefile
DB_URL=postgres://notify:notify@localhost:5432/notification_dispatcher?sslmode=disable

migrate-up:
	goose -dir migrations postgres "$(DB_URL)" up

migrate-down:
	goose -dir migrations postgres "$(DB_URL)" down

migrate-status:
	goose -dir migrations postgres "$(DB_URL)" status
```

### Installing goose CLI

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
goose --version
```

### Running migrations

```bash
make migrate-up
# OK    00001_create_events_table.sql

make migrate-status
# Applied: 00001_create_events_table.sql
```

### Verifying the table was created

```bash
docker run --name notification-db \
  -e POSTGRES_USER=notify \
  -e POSTGRES_PASSWORD=notify \
  -e POSTGRES_DB=notification_dispatcher \
  -p 5432:5432 \
  -d postgres:16
docker exec -it notification-db psql -U notify -d notification_dispatcher -c "\dt"
```

Expected output:
```
 Schema |       Name        | Type  | Owner
--------+-------------------+-------+-------
 public | events            | table | notify
 public | goose_db_version  | table | notify
```

---

## 4. Interview Takeaway

**Why database migrations instead of manual table creation:**
Manual table creation cannot be reproduced, versioned, or rolled back.
Migration files are committed to source control — every environment gets the exact same schema
by running the same files in the same order. New developers run one command and are ready.

**How goose differs from Flyway:**
Same concept — versioned SQL files run in order, history tracked in a DB table.
`goose` is lighter, has a simpler file format, and works well both as a CLI and embedded in Go code.
Flyway uses XML or Java-based config; `goose` uses plain SQL with comment annotations.

**Up and Down migrations:**
Every migration should have both. `Up` applies the change. `Down` reverses it.
Without `Down`, you cannot roll back — which is dangerous in production incidents.

**Where migrations fit in the project lifecycle:**
Migrations run before the application starts serving traffic.
In production: run as a separate step in your deployment pipeline before starting the server.
In local development: run manually via `make migrate-up`.

---

## 5. Cleanup Notes

Day 12 produced permanent production code. Nothing needs to be deleted.

**Keep — permanent project files:**
- `migrations/00001_create_events_table.sql` — schema history, always committed
- `Makefile` — updated with migration targets

**Note:** Every future schema change gets a new numbered migration file.
Never edit an existing migration that has already run — create a new one instead.