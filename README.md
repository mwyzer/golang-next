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

## Stack

- **API / worker**: Go, [chi](https://github.com/go-chi/chi) router, [pgx](https://github.com/jackc/pgx)
- **Database**: PostgreSQL ([pgvector](https://github.com/pgvector/pgvector) for future RAG support)
- **Frontend**: Next.js (App Router), React
- **OCR / LLM**: pluggable `Provider` interfaces; stub implementations ship by default (see [docs/PRD.md](docs/PRD.md) Open Questions)

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
| `API_TOKEN` | `dev-token` | Shared bearer token (see docs/PRD.md Open Questions re: real auth) |
| `POLL_INTERVAL_SECONDS` | `3` | Worker backoff between empty polls |
| `MAX_AGENT_ITERATIONS` | `10` | Cap on pipeline steps per agent run |

The web app reads `NEXT_PUBLIC_API_URL` and `NEXT_PUBLIC_API_TOKEN`.

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
