# Implementation Notes

Guidance for turning the [Technical Design](technical-design/api.md) into
working code for the AI Document Processing Agent.

## Repo Layout

| Path | Purpose |
| --- | --- |
| `cmd/api/` | API server entrypoint |
| `cmd/worker/` | Agent worker entrypoint (polls Postgres for `UPLOADED` documents — no separate queue, see [Tools](technical-design/tools.md)) |
| `internal/domain/` | Core domain types (Document, AgentRun, ReviewTask, ...) |
| `internal/agent/` | Agent loop, tool registry, tool implementations |
| `internal/providers/llm/` | LLM provider interface + implementations |
| `internal/providers/ocr/` | OCR provider interface + implementations |
| `internal/storage/` | Object storage interface (local disk / S3-compatible) |
| `internal/api/` | HTTP handlers, routing, middleware (auth, logging) |
| `internal/db/` | Postgres repositories, hand-written raw SQL via `pgx` (no code generation, no ORM) |
| `db/migrations/` | SQL migrations (see [DB design](technical-design/db.md)) |
| `web/` | Next.js frontend |
| `Dockerfile` (api, worker, web) | Container images per component (NFR-21) |
| `docker-compose.yml` | Local stack: API, worker, web, Postgres (NFR-22) |

## Coding Conventions

- Backend (Go): idiomatic Go project layout (NFR-16); providers (LLM, OCR, storage) implemented behind interfaces so they can be swapped without touching agent/domain code (NFR-17); no package-level global state for request-scoped data.
- Frontend (Next.js): TypeScript strict mode. All three pages are currently client components (`"use client"`) fetching via `web/lib/api.ts`, including the read-heavy upload/review-queue views — no server components in use yet.

## Branching & PR Workflow

- Feature branches off `master`; PRs required before merge.
- CI (`.github/workflows/ci.yml`: build/vet/gofmt/unit tests, integration tests, E2E tests, and the frontend lint/typecheck/test/build job) must pass before merge — see [Testing](TESTING.md).

## Definition of Done

- [ ] Code implemented and matches the relevant [SRS](SRS.md) feature and [Technical Design](technical-design/api.md) contract.
- [ ] Unit tests written and passing; new agent tools have execution tests (success, failure, timeout).
- [ ] Docs updated if behavior or contracts changed.
- [ ] Reviewed and approved.

---

Next stage: [Testing](TESTING.md)
