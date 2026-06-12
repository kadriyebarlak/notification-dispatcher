# Day 29 — Interview Prep: Project Walkthrough

A prepared 3–5 minute verbal walkthrough of the notification dispatcher project,
for the "tell me about a project you built" question that opens most technical interviews.

**This is a map, not a speech.** The interviewer will interrupt with questions —
that is good. Be able to navigate from any point and come back.

---

## The five-part structure

| Part | Time | Purpose |
|---|---|---|
| 1. Context | ~30s | What it does and why you built it |
| 2. Architecture | ~90s | How a request flows through the system |
| 3. A technical decision | ~60s | Show depth and tradeoff thinking |
| 4. What next | ~45s | Self-awareness about limitations |
| 5. What you learned | ~30s | The honest transition story |

---

## Part 1 — Context

I built a notification dispatcher service in Go. It receives notification events over a
REST API and dispatches them asynchronously through different notifier channels like email
and webhook.

In this project I focused on learning idiomatic Go. I deliberately chose a domain I already
knew from my Java work — notification dispatch — so I could focus on the language, not the
problem.

---

## Part 2 — Architecture

The flow is: an HTTP handler receives an event, validates it, and stores it in PostgreSQL
with a pending status — then returns 202 Accepted immediately, so the client does not wait
for delivery.

Separately, a dispatcher polls the database every interval for pending events and feeds them
into a buffered channel in a worker pool. The workers are goroutines that pull events from
the channel and call the right notifier from a registry based on event type. This all happens
concurrently.

Status moves from pending → processing → delivered, with a retry mechanism on failure and a
dead status after max retries.

---

## Part 3 — A technical decision

One decision I am happy with is keeping the worker pool generic. The pool does not know
anything about notifications — it just takes a process function. The actual logic lives in
the dispatcher, which injects it. This keeps the pool reusable and easy to test in isolation.

I also used interfaces at every layer, so I can test each layer with fake implementations —
no real database or network needed.

---

## Part 4 — What next

There are some deliberate limitations. The notifiers are fake implementations right now —
the next step would be wiring in a real email provider.

For scaling to multiple instances, I would change the dispatcher query to use
`SELECT FOR UPDATE SKIP LOCKED`, so two instances do not process the same event.

I also found and fixed a subtle shutdown bug — in-flight jobs were using the cancelled
shutdown context, so their database updates would fail. I separated the processing context
from the shutdown context so running jobs finish cleanly.

---

## Part 5 — What I learned

The biggest shift from Java was that Go has no magic — I wire every dependency myself and
handle every error explicitly.

Dependency injection actually clicked for me here. I had used it for years in Spring through
annotations, but I never fully understood it until I wired it by hand in Go and could see
exactly what it does.

---

## Likely follow-up questions and where to go

The interviewer will dig into parts of the walkthrough. Be ready to expand:

| If they ask about... | Go to... |
|---|---|
| "Tell me more about the worker pool" | goroutines, buffered channel, WaitGroup, the generic process function |
| "How does retry work?" | retry_count, max retries, FindByStatuses for pending + failed, dead status |
| "How do you prevent duplicate processing?" | PROCESSING status set before submit; SELECT FOR UPDATE SKIP LOCKED for multi-instance |
| "How do you test this?" | interfaces + fakes, fake repository records calls, no DB needed |
| "Why Go over Java here?" | 34MB vs 200-400MB image, cheap goroutines, no JVM warm-up |
| "What happens on shutdown?" | signal channel, cancel context, drain HTTP, stop pool, close DB, separate process context |

---

## Delivery reminders

- **It is a map, not a script.** You should be able to start anywhere and come back.
- **Interruptions are good** — they mean the interviewer is engaged.
- **Explain the why, not just the what.** "Returns 202 immediately so the client does not wait"
  is more impressive than "stores it in the database."
- **Mention the bug you caught.** Most candidates never mention bugs they found and fixed.
  It signals production-level thinking.
- **The DI story is your strongest moment.** "I used it for years in Spring but only understood
  it in Go" is honest, specific, and memorable.
- **Practice out loud with a timer.** Reading and speaking are different skills.
  The third time through is always smoother than the first.