# Testing Strategy

Verifies the [Implementation](IMPLEMENTATION.md) against the
[SRS](SRS.md) traceability matrix.

## Test Levels

| Level | Scope | Tooling |
| --- | --- | --- |
| Unit | Domain logic, individual agent tools (mocked LLM/OCR), validation rules | Go `testing` + `testify`; Frontend: Jest/Vitest |
| Integration | API endpoints against a real Postgres (testcontainers), job queue processing | Go `testing` + `testcontainers-go` |
| Agent | Full agent-run scenarios: happy path, low confidence → review, validation failure, iteration cap exceeded, tool timeout | Go, with a fake/stubbed LLM+OCR provider |
| End-to-End | Upload → process → review → completed, through the API (and optionally the UI) | Playwright or Go E2E harness |

## Coverage Targets

- Domain and agent packages: 80%+ line coverage.
- Every tool in the [tool registry](architecture/agent-architecture.md) has at least one success-path and one failure-path test.
- Every acceptance criterion in the [SRS](SRS.md) maps to at least one automated test (see Traceability Matrix).

## Test Environments

| Environment | Purpose |
| --- | --- |
| Local | Developer machine; Docker Compose for Postgres + queue |
| CI | Ephemeral containers per PR run |
| Staging | Pre-production, using real (non-production) provider credentials |

## CI Gate

- Lint, unit tests, and integration tests must pass before merge.
- Agent-run tests must pass with zero unbounded loops (iteration cap enforced) and zero unregistered tool calls.

---

Next stage: [Deployment](DEPLOYMENT.md)
