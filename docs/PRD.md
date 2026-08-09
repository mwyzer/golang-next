# Product Requirements Document (PRD)

## Summary

AI Document Processing Agent is an agentic document intelligence platform
that automatically classifies, extracts, validates, and processes business
documents such as invoices, receipts, and resumes. The system uses AI agents,
OCR, structured extraction, RAG, and business tools to reduce manual document
processing while routing uncertain cases to human reviewers.

## Goals

- Automatically classify uploaded documents.
- Extract structured information from supported documents.
- Validate extracted information.
- Calculate extraction confidence.
- Automatically process high-confidence documents.
- Route low-confidence documents to human reviewers.
- Provide an auditable processing history.
- Demonstrate Agentic AI capabilities using Go.

## Non-Goals

- Full enterprise document management system.
- Supporting every possible document type.
- Fully autonomous processing of high-risk documents.
- Replacing human reviewers entirely.
- Building a custom OCR model.

## Target Users

- Finance staff processing invoices and receipts.
- HR staff processing resumes.
- Operations staff processing business documents.
- Administrators managing document workflows.
- Reviewers handling low-confidence documents.

## User Stories

| As a | I want to | So that |
| ---- | --------- | ------- |
| Finance staff | Upload an invoice | I can process it automatically |
| Finance staff | See extracted invoice data | I can verify the information |
| Reviewer | Review low-confidence documents | I can correct AI extraction |
| HR staff | Upload a CV | Candidate information can be extracted automatically |
| Administrator | Configure document workflows | Different document types can follow different processes |
| Administrator | View audit logs | I can track what the AI agent did |

All of the above are implemented end-to-end (upload → OCR → classify →
extract → validate → confidence → auto-process/route to review → audit
log) — see [Requirements](REQUIREMENTS.md) for the FR breakdown and
[Implementation](IMPLEMENTATION.md) for status per requirement.

## Success Metrics

| Metric | Target |
| ------ | ------ |
| Document classification accuracy | >= 95% |
| Required-field extraction accuracy | >= 90% |
| High-confidence auto-processing rate | >= 70% |
| Invalid tool call rate | < 1% |
| Processing failure rate | < 5% |
| Human review routing accuracy | >= 90% |

Not yet measured — there's no metrics/observability pipeline reporting
against these targets today (`agent_runs.trace_id` exists in the schema
but isn't threaded into log lines yet; see [Tools](technical-design/tools.md)).

## Constraints & Assumptions

### Constraints

- Backend must be implemented in Go.
- The system SHALL be containerized using Docker.
- Local development SHALL be reproducible using Docker Compose.
- PostgreSQL is the primary database.
- The system must support asynchronous document processing.
- Uploaded files must have configurable size limits.
- Agent actions must be restricted to registered tools.
- High-risk actions require human approval.

### Assumptions

- An external LLM provider will be used initially.
- OCR may be provided by an external or local OCR engine.
- Documents are primarily business documents.
- Human reviewers are available for exception handling.
- Initial MVP supports invoice, receipt, and CV documents.

## Open Questions

- ~~Which LLM provider should be used for the MVP?~~ **Resolved:** Anthropic
  Claude. `llm.AnthropicProvider` (`internal/providers/llm/anthropic.go`)
  implements `classify_document`/`extract_fields` via the Messages API,
  forcing a single tool call per request so results are structured JSON.
  It's opt-in via `LLM_PROVIDER=anthropic` + `ANTHROPIC_API_KEY` (worker
  falls back to `llm.StubProvider` — the default — when unset); see
  [Tools](technical-design/tools.md) and the config table in
  [README.md](../README.md).
- ~~Which OCR provider should be used?~~ **Resolved:** also Anthropic
  Claude, via its vision input, rather than a separate OCR vendor —
  `ocr.AnthropicProvider` (`internal/providers/ocr/anthropic.go`) sends
  the stored file's bytes as a PDF or image content block and asks
  Claude to transcribe it, reusing the same `ANTHROPIC_API_KEY`/
  `ANTHROPIC_MODEL` as the LLM provider. Opt-in via `OCR_PROVIDER=anthropic`
  (worker falls back to `ocr.StubProvider` — the default — when unset).
- Should document storage use local storage or S3-compatible storage?
  **Still open as a decision**, though only local disk storage exists
  today (`internal/storage/local.go`, `STORAGE_ROOT`) — no S3-compatible
  backend has been written.
- Which document types should be prioritized after the MVP? **Still
  open.** No types beyond invoice/receipt/cv exist.
- Should the system support multi-tenancy? **Still open.** `tenant_id`
  is threaded through the schema and auth path (every query is
  tenant-scoped), but only the one seeded dev tenant is ever used —
  there's no tenant provisioning flow or a second tenant to test
  isolation against.
- What actions require mandatory human approval? **Still open as an
  explicit policy**, though the routing gates that exist today (unknown
  type, validation failure, duplicate, low confidence → `PENDING_REVIEW`)
  already function as a de facto answer for those specific cases.

---

Next stage: [Requirements](REQUIREMENTS.md)