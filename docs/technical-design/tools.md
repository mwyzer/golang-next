# Tools & Integrations

Supporting tooling, libraries, and third-party integrations required by
the [Technical Design](api.md). This is distinct from the AI Agent's
[tool registry](../architecture/agent-architecture.md) — this doc covers
engineering tooling, not agent-callable tools.

## Backend (Go)

| Tool/Library | Purpose | Notes |
| --- | --- | --- |
| `chi` or `gin` | HTTP router/API framework | Pick one; keep handlers thin |
| `pgx` | PostgreSQL driver | Used directly or via `sqlx` |
| `golang-migrate` or `atlas` | Schema migrations | See [DB design](db.md) |
| `river` or `asynq` | Background job queue for async processing | Satisfies FR-26, NFR-15 |
| `zerolog` or `slog` | Structured logging with trace IDs | Satisfies NFR-18, NFR-19 |
| `testify` | Unit/integration test assertions and mocks | |
| LLM SDK (provider-specific) | Classification, extraction, reasoning calls | Abstracted behind a provider interface (NFR-17) |
| OCR client (provider-specific or local) | Text extraction from images/PDFs | Abstracted behind a provider interface (NFR-17) |

## Frontend (Next.js)

| Tool/Library | Purpose | Notes |
| --- | --- | --- |
| Next.js (TypeScript) | Upload UI, status views, review queue | App Router recommended |
| A data-fetching library (e.g. TanStack Query) | Polling document status, mutations for review actions | |
| A UI component library | Forms, tables, review screens | Choice open |

## External Services

| Service | Purpose | Auth Method |
| --- | --- | --- |
| LLM Provider (TBD — see PRD Open Questions) | Classification, structured extraction, reasoning | API key |
| OCR Provider (TBD — see PRD Open Questions) | Text extraction from scanned/image documents | API key or local |
| Object Storage (local disk / S3-compatible) | Original file storage | IAM credentials / local filesystem |

## Dev Tooling

| Tool | Purpose |
| --- | --- |
| Linting | `golangci-lint` (Go), ESLint (Next.js) |
| Formatting | `gofmt`/`goimports` (Go), Prettier (Next.js) |
| CI | Run lint, unit tests, and build on every PR (see [Testing](../TESTING.md)) |
| Docker | Containerizes the API server, agent worker, and frontend (NFR-21) |
| Docker Compose | Runs the full local stack (API, worker, frontend, Postgres, queue) with one command (NFR-22) |

---

See also: [API](api.md) · [DB](db.md)
Next stage: [Implementation](../IMPLEMENTATION.md)
