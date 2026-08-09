# AI Document Processing Agent

[![CI](https://github.com/mwyzer/golang-next/actions/workflows/ci.yml/badge.svg)](https://github.com/mwyzer/golang-next/actions/workflows/ci.yml)

An agentic document intelligence platform that classifies, extracts,
validates, and processes business documents (invoices, receipts,
resumes) — routing anything uncertain to a human reviewer instead of
guessing. Backend in Go, frontend in Next.js.

See [docs/PRD.md](docs/PRD.md) for the full product summary and goals.

## How it works

1. **Upload** — a client (or the web UI) `POST`s a PDF/PNG/JPEG to the
   API. The document is stored and a row is created with status
   `UPLOADED`; the request returns immediately.
2. **Process** — a worker polls for `UPLOADED` documents and runs each
   one through an agent pipeline: OCR → classify → extract fields →
   validate → check for duplicates → score confidence → finalize.
3. **Route** — a document either auto-processes (high confidence, no
   issues) or gets routed to a human reviewer, with the reason recorded
   (unknown type, validation failure, duplicate, low confidence).
4. **Review** — reviewers approve, reject, or correct extracted fields
   through the web UI; every decision is written to an append-only
   audit log.

Full design docs: [docs/architecture/agent-architecture.md](docs/architecture/agent-architecture.md),
[docs/architecture/system-architecture.md](docs/architecture/system-architecture.md).

```mermaid
sequenceDiagram
    actor U as User
    participant W as Web UI
    participant A as API
    participant DB as PostgreSQL
    participant Wk as Worker
    actor R as Reviewer

    U->>W: Select file
    W->>A: POST /documents
    A->>DB: INSERT document (status = UPLOADED)
    A-->>W: 201 {document_id, status: UPLOADED}

    loop poll with backoff
        Wk->>DB: claim oldest UPLOADED document (SKIP LOCKED)
    end
    Wk->>Wk: run_ocr -> classify -> extract -> validate -> check_duplicate -> calculate_confidence
    alt cleared every gate
        Wk->>DB: status = AUTO_PROCESSED
    else routed to review
        Wk->>DB: status = PENDING_REVIEW, create review_task
    end

    W->>A: GET /documents/{id}
    A-->>W: current status

    opt needs review
        R->>W: Open review queue
        W->>A: GET /review-queue
        A-->>W: pending review tasks
        R->>W: Approve / Reject / Correct
        W->>A: POST /documents/{id}/review
        A->>DB: resolve review_task, status = REVIEWED
        A-->>W: 200
    end
```

## Stack

- **API / worker**: Go, [chi](https://github.com/go-chi/chi) router, [pgx](https://github.com/jackc/pgx)
- **Database**: PostgreSQL ([pgvector](https://github.com/pgvector/pgvector) for future RAG support)
- **Frontend**: Next.js (App Router), React
- **OCR / LLM**: pluggable `Provider` interfaces; stub implementations ship by default (see [docs/PRD.md](docs/PRD.md) Open Questions)

## Architecture

**System context** — see [docs/architecture/system-architecture.md](docs/architecture/system-architecture.md)
for the full component breakdown. There's no separate job queue: the
worker claims documents by polling Postgres directly with
`SELECT ... FOR UPDATE SKIP LOCKED`, which is enough to safely dequeue
across multiple worker instances at this scale.

```mermaid
flowchart LR
    User((User)) --> Frontend[Next.js Frontend]
    Reviewer((Reviewer)) --> Frontend
    Frontend --> API[Go API Server]
    API --> DB[(PostgreSQL)]
    Worker[Go Agent Worker] -->|polls for UPLOADED documents| DB
    Worker --> LLM[[LLM Provider]]
    Worker --> OCR[[OCR Provider]]
    Worker -.future.-> VectorDB[(pgvector RAG Store)]
    API --> Storage[(Object Storage)]
    Worker --> Storage
```

**Agent pipeline** — see [docs/architecture/agent-architecture.md](docs/architecture/agent-architecture.md)
for the tool registry and guardrails (iteration cap, tool allowlist,
timeouts). Routing to `PENDING_REVIEW` is a normal, successful outcome;
only an unrecoverable error or exceeding the iteration cap fails the
run.

```mermaid
flowchart TD
    Start([Document claimed: UPLOADED]) --> OCR[run_ocr]
    OCR --> Classify[classify_document]
    Classify -->|type = unknown| Review([PENDING_REVIEW])
    Classify -->|classified| Extract[extract_fields]
    Extract --> Validate[validate_extraction]
    Validate -->|violations found| Review
    Validate -->|valid| Dedup[check_duplicate]
    Dedup -->|exact or near-duplicate match| Review
    Dedup -->|unique| Confidence[calculate_confidence]
    Confidence -->|below auto-process threshold| Review
    Confidence -->|>= threshold| Finalize[finalize_document]
    Finalize --> Done([AUTO_PROCESSED])

    OCR -.error, or iteration cap exceeded.-> Failed([FAILED])
    Classify -.error, or iteration cap exceeded.-> Failed
    Extract -.error, or iteration cap exceeded.-> Failed
    Validate -.iteration cap exceeded.-> Failed
    Dedup -.error, or iteration cap exceeded.-> Failed
    Confidence -.error, or iteration cap exceeded.-> Failed
    Finalize -.error, or iteration cap exceeded.-> Failed
```

## Repo layout

```text
cmd/api/         API server entrypoint
cmd/worker/      Background worker entrypoint (polls and runs the agent pipeline)
internal/agent/  Agent pipeline (Runner)
internal/api/    HTTP handlers, routing, middleware
internal/db/     Postgres repositories
internal/domain/ Shared domain types
internal/providers/  OCR and LLM provider interfaces + stubs
internal/storage/    Document storage abstraction (local disk / S3-compatible)
internal/validation/ Extraction validation rules
db/migrations/   SQL schema migrations (embedded into the binary)
web/             Next.js frontend
e2e/             End-to-end tests (real API + worker loop)
docs/            SDLC docs: PRD, requirements, SRS, architecture, technical design
```

## Data model

```mermaid
erDiagram
    tenants ||--o{ users : has
    tenants ||--o{ documents : owns
    tenants ||--o{ audit_logs : scopes
    tenants ||--o{ knowledge_chunks : owns
    users ||--o{ documents : uploads
    users ||--o{ review_tasks : reviews
    document_types ||--o{ documents : classifies
    documents ||--o{ extracted_fields : has
    documents ||--o{ agent_runs : processed_by
    documents ||--o{ review_tasks : may_have
    agent_runs ||--o{ tool_executions : contains

    documents {
        uuid id PK
        uuid tenant_id FK
        uuid uploaded_by FK
        text document_type_id FK "null until classified"
        text status
        text content_hash "exact-duplicate lookup"
        text key_fields_hash "near-duplicate lookup, nullable"
        numeric classification_confidence
        numeric overall_confidence
        boolean is_duplicate
        uuid duplicate_of_document_id FK "self-reference, nullable"
    }
    review_tasks {
        uuid id PK
        uuid document_id FK
        text reason "LOW_CONFIDENCE, VALIDATION_FAILED, DUPLICATE, or UNKNOWN_TYPE"
        text status "PENDING, APPROVED, REJECTED, or CORRECTED"
    }
    agent_runs {
        uuid id PK
        uuid document_id FK
        text status "RUNNING, COMPLETED, or FAILED"
        int iteration_count
        int max_iterations
    }
```

Full column-level schema (every table, every field): [docs/technical-design/db.md](docs/technical-design/db.md).

## Getting started

### Docker Compose (recommended)

```bash
cp .env.example .env
docker compose up --build
```

- API: <http://localhost:8080> (health check: `GET /healthz`)
- Web UI: <http://localhost:3000>
- Postgres: `localhost:5433` (user/pass/db: `docagent`)

### Running natively

```bash
# Postgres only, via compose
docker compose up postgres

# API (in one terminal)
go run ./cmd/api

# Worker (in another terminal)
go run ./cmd/worker

# Web UI
cd web && npm install && npm run dev
```

Default config (see `internal/config/config.go`) points the API/worker
at `postgres://docagent:docagent@localhost:5432/docagent` — override
`DATABASE_URL` to match Compose's mapped port (`5433`) if running
outside Docker:

```bash
export DATABASE_URL="postgres://docagent:docagent@localhost:5433/docagent?sslmode=disable"
```

### Configuration

| Env var | Default | Purpose |
| --- | --- | --- |
| `PORT` | `8080` | API listen port |
| `DATABASE_URL` | `postgres://docagent:docagent@localhost:5432/docagent?sslmode=disable` | Postgres connection string |
| `STORAGE_ROOT` | `/data/uploads` | Local document storage root |
| `MAX_UPLOAD_BYTES` | `20971520` (20 MB) | Max upload size |
| `API_TOKEN` | `dev-token` | Bootstraps the seeded dev user's per-user bearer token on API startup (see [Authentication](#authentication)) |
| `POLL_INTERVAL_SECONDS` | `3` | Worker backoff between empty polls |
| `MAX_AGENT_ITERATIONS` | `10` | Cap on pipeline steps per agent run |
| `TOOL_TIMEOUT_SECONDS` | `30` | Timeout for each OCR/LLM provider call; `0` disables it |
| `MAX_RETRIES` | `2` | Bounded retries for a recoverable agent-run failure (3 total attempts); `0` disables retry |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Comma-separated origins allowed to call the API from a browser (the web UI's origin) |
| `LLM_PROVIDER` | `stub` | Worker only. `stub` (keyword/heuristic) or `anthropic` (real Claude calls via `github.com/anthropics/anthropic-sdk-go`, see `internal/providers/llm/anthropic.go`) |
| `OCR_PROVIDER` | `stub` | Worker only. `stub` (reads raw file bytes as text) or `anthropic` (real OCR via Claude vision, see `internal/providers/ocr/anthropic.go`) |
| `ANTHROPIC_API_KEY` | *(unset)* | Worker only. Required when `LLM_PROVIDER=anthropic` and/or `OCR_PROVIDER=anthropic` (shared key) — the worker fails to start without it |
| `ANTHROPIC_MODEL` | `claude-opus-5` | Worker only. Model ID used for both providers above when set to `anthropic` |

The web app reads `NEXT_PUBLIC_API_URL` and `NEXT_PUBLIC_API_TOKEN`.

## Authentication

Every user has their own opaque bearer token — not a JWT, not a
session cookie, and not one token shared by everyone. Only each
token's SHA-256 hash is persisted (`users.token_hash`), so a database
leak doesn't directly expose usable credentials. `RequireAuth` looks
up the token's hash and authenticates the request as that specific
user (their own tenant ID, user ID, and role), not a fixed shared
identity.

There's no signup or login flow. Two ways to get a token:

- **The seeded dev user**: on every API startup, `cmd/api/main.go`
  sets (or rotates) the dev user's token to whatever `API_TOKEN` is
  currently set to (default `dev-token`) — this is what the `curl`
  examples below and the web UI's `NEXT_PUBLIC_API_TOKEN` use.
- **New users**: an admin calls `POST /api/v1/users` with an email and
  role (`uploader`, `reviewer`, or `admin`); the response includes the
  new user's token, shown exactly once:

  ```bash
  curl -X POST http://localhost:8080/api/v1/users \
    -H "Authorization: Bearer dev-token" \
    -H "Content-Type: application/json" \
    -d '{"email": "reviewer@example.com", "role": "reviewer"}'
  # {"user_id": "...", "email": "reviewer@example.com", "role": "reviewer", "token": "..."}
  ```

There's still only one tenant in practice (the seeded dev tenant) —
multi-tenancy is a separate, unbuilt feature (see [docs/PRD.md](docs/PRD.md) Open Questions).

## Operating with Docker Compose

Migrations run automatically on startup (`db.Migrate` is called from
both `cmd/api/main.go` and `cmd/worker/main.go`) — there's no separate
migrate step.

| Task | Command |
| --- | --- |
| Start (foreground, see logs) | `docker compose up` |
| Start (background/detached) | `docker compose up -d` |
| Stop | `docker compose down` |
| Stop **and wipe data** (Postgres volume, uploaded files) | `docker compose down -v` |
| Rebuild after code changes | `docker compose up --build` |
| Rebuild just one service | `docker compose build worker && docker compose up -d worker` |
| Restart one service (no rebuild) | `docker compose restart api` |
| Tail logs, all services | `docker compose logs -f` |
| Tail logs, one service | `docker compose logs -f worker` |
| Check what's running | `docker compose ps` |
| Scale the worker | `docker compose up -d --scale worker=3` |

The worker claims documents via `SELECT ... FOR UPDATE SKIP LOCKED`,
so running more than one instance is safe.

```bash
# health check
curl http://localhost:8080/healthz

# upload a document (dev-token is the default API_TOKEN)
curl -X POST http://localhost:8080/api/v1/documents \
  -H "Authorization: Bearer dev-token" \
  -F "file=@invoice.pdf"

# check status
curl http://localhost:8080/api/v1/documents/<document_id> \
  -H "Authorization: Bearer dev-token"

# connect to Postgres directly
docker compose exec postgres psql -U docagent -d docagent
```

`worker` exposes no port — it's a pure background poller, nothing to
curl. `web`'s `NEXT_PUBLIC_API_URL`/`NEXT_PUBLIC_API_TOKEN` are baked
in at container start; if you change `API_TOKEN` in `.env`, update
those too or the UI's requests will get `401`s. Uploaded files persist
in the `uploads` named volume across restarts — only
`docker compose down -v` clears them.

## Testing

```bash
go test ./...                        # Go unit tests (fast, no Docker)
go test -tags=integration ./...      # + internal/db, internal/agent, internal/api against testcontainers
go test -tags=e2e ./e2e/... -v       # end-to-end: upload -> process -> review -> completed

cd web
npm run lint && npx tsc --noEmit && npm test && npm run build
```

Integration and E2E tests need Docker (they start a `pgvector/pgvector:pg16`
container via [testcontainers-go](https://github.com/testcontainers/testcontainers-go)).
See [docs/TESTING.md](docs/TESTING.md) for the full test-level breakdown
and what `.github/workflows/ci.yml` runs on every push/PR.

## Documentation

| Doc | Purpose |
| --- | --- |
| [docs/SDLC.md](docs/SDLC.md) | How these docs relate to each other |
| [docs/PRD.md](docs/PRD.md) | Product requirements |
| [docs/REQUIREMENTS.md](docs/REQUIREMENTS.md) | Functional / non-functional requirements |
| [docs/SRS.md](docs/SRS.md) | Formalized, testable requirements + traceability matrix |
| [docs/architecture/system-architecture.md](docs/architecture/system-architecture.md) | System architecture |
| [docs/architecture/agent-architecture.md](docs/architecture/agent-architecture.md) | Agent pipeline design |
| [docs/architecture/data-architecture.md](docs/architecture/data-architecture.md) | Data architecture |
| [docs/technical-design/api.md](docs/technical-design/api.md) | API design |
| [docs/technical-design/db.md](docs/technical-design/db.md) | Database schema |
| [docs/technical-design/tools.md](docs/technical-design/tools.md) | Agent tool registry |
| [docs/IMPLEMENTATION.md](docs/IMPLEMENTATION.md) | Implementation notes |
| [docs/TESTING.md](docs/TESTING.md) | Testing strategy |
| [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) | Deployment |
| [docs/OPERATIONS.md](docs/OPERATIONS.md) | Operations |
