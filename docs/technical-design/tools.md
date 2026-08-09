# Tools & Integrations

Supporting tooling, libraries, and third-party integrations required by
the [Technical Design](api.md). This is distinct from the AI Agent's
[tool registry](../architecture/agent-architecture.md) — this doc covers
engineering tooling, not agent-callable tools.

## Backend (Go)

| Tool/Library | Purpose | Notes |
| --- | --- | --- |
| `chi` | HTTP router/API framework | Decided |
| `pgx` | PostgreSQL driver | Used directly (no ORM/query builder) |
| Custom embedded-SQL runner (`internal/db/migrate.go`) | Schema migrations | Not `golang-migrate`/`atlas` — a small custom runner applies `db/migrations/*.up.sql` in filename order via a `schema_migrations` table, embedded into the binary with `//go:embed`. Runs automatically on `cmd/api`/`cmd/worker` startup. |
| No separate queue library | Async processing | Satisfies FR-26, NFR-15 without a dedicated queue: the worker claims the oldest `UPLOADED` document with `SELECT ... FOR UPDATE SKIP LOCKED` directly against Postgres — see [System Architecture](../architecture/system-architecture.md) for why a queue (`river`/`asynq`/Redis) wasn't needed at this scale |
| Standard library `log` | Logging | Not `zerolog`/`slog` — plain `log.Printf`, correlated by agent run ID (`agent_runs.trace_id` exists in the schema but isn't yet threaded into log lines; partially satisfies NFR-18/NFR-19) |
| `testify` | Unit/integration test assertions | `assert`/`require` only, no mocking — integration tests run against a real Postgres via `testcontainers-go` |
| `github.com/anthropics/anthropic-sdk-go` | Classification, extraction, reasoning calls | Abstracted behind `llm.Provider`. `llm.StubProvider` (keyword/heuristic) is the default; `llm.AnthropicProvider` (`internal/providers/llm/anthropic.go`) is a real Claude-backed implementation, opt-in via `LLM_PROVIDER=anthropic` + `ANTHROPIC_API_KEY` (`llm.NewProvider` picks between them — see `cmd/worker/main.go`). Resolves the LLM-provider PRD Open Question; satisfies NFR-17 (swappable behind the interface) |
| `github.com/anthropics/anthropic-sdk-go` (shared with the LLM row above) | Text extraction from images/PDFs | Abstracted behind `ocr.Provider`. `ocr.StubProvider` (reads raw file bytes as text) is the default; `ocr.AnthropicProvider` (`internal/providers/ocr/anthropic.go`) sends the file to Claude as a PDF/image content block and returns its transcription — no separate OCR vendor, opt-in via `OCR_PROVIDER=anthropic` (`ocr.NewProvider` picks between them). Resolves the OCR-provider PRD Open Question; satisfies NFR-17 |

## Frontend (Next.js)

| Tool/Library | Purpose | Notes |
| --- | --- | --- |
| Next.js (TypeScript), App Router | Upload UI, status views, review queue | Decided |
| Plain `fetch` (`web/lib/api.ts`) | Polling document status, mutations for review actions | No data-fetching library (e.g. TanStack Query) — manual `useState`/`useEffect` |
| None | Forms, tables, review screens | Plain inline styles, no UI component library |

## External Services

| Service | Purpose | Auth Method |
| --- | --- | --- |
| Anthropic Claude (opt-in — see `LLM_PROVIDER`) | Classification, structured extraction, reasoning | `ANTHROPIC_API_KEY` |
| Anthropic Claude (opt-in — see `OCR_PROVIDER`) | Text extraction from scanned/image documents, via vision input | `ANTHROPIC_API_KEY` (shared with the row above) |
| Object Storage (local disk / S3-compatible) | Original file storage | IAM credentials / local filesystem |

## Dev Tooling

| Tool | Purpose |
| --- | --- |
| Linting | `go vet` + `gofmt -l` (Go — no `golangci-lint` configured); ESLint via `next lint` (Next.js) |
| Formatting | `gofmt` (Go), no Prettier config (Next.js) |
| CI | `.github/workflows/ci.yml`: build/vet/gofmt/unit tests, `testcontainers-go` integration tests, E2E tests, and the Next.js lint/typecheck/test/build job on every push/PR (see [Testing](../TESTING.md)) |
| Docker | Containerizes the API server, agent worker, and frontend (NFR-21) |
| Docker Compose | Runs the full local stack (API, worker, frontend, Postgres) with one command — no separate queue service (NFR-22) |

---

See also: [API](api.md) · [DB](db.md)
Next stage: [Implementation](../IMPLEMENTATION.md)
