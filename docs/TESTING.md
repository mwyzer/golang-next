# Testing Strategy

Verifies the [Implementation](IMPLEMENTATION.md) against the
[SRS](SRS.md) traceability matrix.

## Test Levels

| Level | Scope | Tooling | Status |
| --- | --- | --- | --- |
| Unit | Validation rules, LLM stub provider, upload MIME/size validation, API client + upload/review-queue/review-detail pages | Go `testing` + `testify`; Vitest + React Testing Library | `internal/validation`, `internal/providers/llm`, `internal/api` (`validate_test.go`); `web/lib/api.test.ts`, `web/app/**/*.test.tsx` |
| Integration | Every `internal/db` repo, plus API endpoints (upload/get/review/queue/auth) against a real Postgres | Go `testing` + `testcontainers-go` | `internal/db/integration_test.go`, `internal/api/integration_test.go` |
| Agent | Full agent-run scenarios: auto-process happy path, all four review-routing reasons (unknown type, validation failed, duplicate, low confidence), max-iterations exceeded, OCR failure | Go, with a fake `llm.Provider`/`ocr.Provider` | `internal/agent/runner_integration_test.go` |
| End-to-End | Upload → process → review → completed, through the real HTTP API plus a real worker polling loop (production stub OCR/LLM providers, not test fakes) | Go `testing` + `testcontainers-go` | `e2e/e2e_test.go` |

Frontend tests (`web/`) mock `@/lib/api` (or `fetch`, for `api.test.ts`
itself) — no server or Postgres involved. Run with `npm test` from
`web/`. The React Testing Library setup (`web/vitest.setup.ts`) registers
`afterEach(cleanup)` explicitly since Vitest isn't run in `globals` mode.

Integration, agent, and E2E tests are gated behind build tags
(`//go:build integration` / `//go:build e2e`) so `go test ./...` stays
fast and Docker-free; run them explicitly with
`go test -tags=integration ./...` or `go test -tags=e2e ./e2e/...`.
They share one Postgres testcontainer per test binary
(`internal/testutil`, started once in each package's `TestMain`) using
the same `pgvector/pgvector:pg16` image as `docker-compose.yml`, since
the schema migration requires the `vector` extension. Each test resets
the mutable tables (`documents`, `extracted_fields`, `agent_runs`,
`tool_executions`, `review_tasks`, `audit_logs`, `knowledge_chunks`)
before running, while leaving the seeded reference data (dev tenant,
dev user, document types) in place.

The E2E suite (`e2e/e2e_test.go`) doesn't spin up `cmd/api`/`cmd/worker`
as separate OS processes or use Docker Compose — it wires the same
`api.NewRouter` and `agent.Runner` those binaries use into an
`httptest.Server` plus a background polling goroutine, using the real
`ocr.StubProvider`/`llm.StubProvider` (not test fakes). This exercises
the full async pipeline — upload via HTTP, the worker loop picking the
document up and processing it, status changes becoming visible via
GET — without the cost of driving a browser or orchestrating containers
for every service.

## Coverage Targets

- Domain and agent packages: 80%+ line coverage.
- Every tool in the [tool registry](architecture/agent-architecture.md) has at least one success-path and one failure-path test.
- Every acceptance criterion in the [SRS](SRS.md) maps to at least one automated test (see Traceability Matrix).

## Test Environments

| Environment | Purpose |
| --- | --- |
| Local | Developer machine; `go test -tags=integration ./...` starts its own testcontainer (Docker Compose only needed for running the app itself) |
| CI | GitHub Actions (`.github/workflows/ci.yml`), ephemeral testcontainers per run |
| Staging | Pre-production, using real (non-production) provider credentials |

## CI Gate

`.github/workflows/ci.yml` runs four jobs on every push/PR to `master`:

- **go**: `go build`, `gofmt -l` (fails on unformatted files), `go vet`, `go test ./...` (unit tests).
- **go-integration**: `go test -tags=integration ./...` — the full `internal/db`, `internal/agent`, and `internal/api` suites against real testcontainers.
- **go-e2e**: `go test -tags=e2e ./e2e/...` — upload → process → review → completed through the real API and worker loop.
- **web**: `npm ci`, `npm run lint`, `npx tsc --noEmit`, `npm test`, `npm run build`.

Agent-run tests must pass with zero unbounded loops (iteration cap
enforced, see `TestRunner_MaxIterationsExceeded`) and zero unregistered
tool calls.

---

Next stage: [Deployment](DEPLOYMENT.md)
