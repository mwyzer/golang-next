# Software Requirements Specification (SRS)

Formalized, testable version of [Requirements](REQUIREMENTS.md), ready to
drive [Architecture](architecture/system-architecture.md) decisions.

## Scope

The AI Document Processing Agent is a backend-first document intelligence
platform that uses AI agents to classify, extract, validate, and process
business documents.

The MVP supports:

- Invoice documents
- Receipt documents
- CV / Resume documents
- PDF, PNG, JPG, and JPEG files
- OCR-based text extraction
- LLM-based structured extraction
- Document validation
- Confidence scoring
- Duplicate detection
- Human-in-the-loop review
- Agent tool calling
- Audit logging
- Asynchronous document processing

The system SHALL be designed with tenant isolation in mind so that it can
evolve into a multi-tenant SaaS platform without requiring a fundamental
rewrite of the core domain model.

The MVP does not include subscription billing, payment processing, or
enterprise tenant administration.

## Definitions & Acronyms

| Term | Definition |
| ---- | ---------- |
| AI Agent | Software component capable of reasoning, selecting tools, and executing multi-step workflows. |
| Document | Uploaded file submitted for AI processing. |
| OCR | Optical Character Recognition used to extract text from image-based documents. |
| RAG | Retrieval-Augmented Generation used to provide external knowledge to the LLM. |
| LLM | Large Language Model used for classification, extraction, reasoning, or generation. |
| Tool | A registered function that an AI Agent can invoke to perform a specific operation. |
| Agent Run | A single execution of an AI Agent workflow. |
| Human Review | Manual review performed when automated processing cannot meet configured requirements. |
| Confidence Score | Numeric estimate representing confidence in an extracted field or processing result. |
| Tenant | An organization or logical customer boundary within the SaaS platform. |
| RBAC | Role-Based Access Control. |
| MVP | Minimum Viable Product. |
| PII | Personally Identifiable Information. |

## System Features

### Feature: Document Upload

- **Description:**
  The system SHALL allow authenticated users to upload supported documents.

- **Inputs:**
  - PDF
  - PNG
  - JPG
  - JPEG
  - Maximum configurable file size

- **Processing:**
  1. Authenticate the user.
  2. Authorize access to the tenant/workspace.
  3. Validate file extension and MIME type.
  4. Validate file size.
  5. Generate a unique document ID.
  6. Store the original document.
  7. Create a document record.
  8. Set processing status to `UPLOADED`.

- **Outputs:**
  ```json
  {
    "document_id": "DOC-001",
    "status": "UPLOADED"
  }
  ```

- **Acceptance Criteria:**
  - [ ] Rejects unsupported file extensions/MIME types with `400`.
  - [ ] Rejects files exceeding the configured size limit with `413`.
  - [ ] Rejects unauthenticated requests with `401`.
  - [ ] Rejects requests for tenants the user cannot access with `403`.
  - [ ] Returns a unique `document_id` and status `UPLOADED` on success.
  - [ ] Persists the original file and a document record before responding.

### Feature: Document Classification

- **Description:**
  The AI Agent SHALL classify an uploaded document into one of the
  supported document types before extraction begins.

- **Inputs:**
  - `document_id`
  - OCR text / document image

- **Processing:**
  1. Load the document and run OCR if text is not already available.
  2. Invoke the `classify_document` tool with the extracted text.
  3. Receive a predicted document type and classification confidence.
  4. Persist the classification result.
  5. Set processing status to `CLASSIFIED`, or `FAILED` if classification
     cannot be completed.

- **Outputs:**
  ```json
  {
    "document_id": "DOC-001",
    "document_type": "invoice",
    "classification_confidence": 0.97
  }
  ```

- **Acceptance Criteria:**
  - [ ] Documents are classified into one of: `invoice`, `receipt`, `cv`, or `unknown`.
  - [ ] `unknown` classifications are routed to human review.
  - [ ] Classification confidence is persisted alongside the result.
  - [ ] Classification failures set status to `FAILED` with an error reason.

### Feature: Structured Extraction

- **Description:**
  The AI Agent SHALL extract structured fields from a classified document
  according to the schema defined for its document type.

- **Inputs:**
  - `document_id`
  - `document_type`
  - OCR text / document image

- **Processing:**
  1. Load the field schema for the classified document type.
  2. Invoke the `extract_fields` tool.
  3. Receive extracted field values with per-field confidence scores.
  4. Persist extracted fields.
  5. Set processing status to `EXTRACTED`.

- **Outputs:**
  ```json
  {
    "document_id": "DOC-001",
    "fields": {
      "vendor_name": { "value": "Acme Corp", "confidence": 0.95 },
      "total_amount": { "value": 129.99, "confidence": 0.92 },
      "invoice_date": { "value": "2026-08-01", "confidence": 0.9 }
    }
  }
  ```

- **Acceptance Criteria:**
  - [ ] Extraction output matches the field schema for the document's type.
  - [ ] Every extracted field has an associated confidence score.
  - [ ] Missing required fields are flagged rather than silently omitted.

### Feature: Validation

- **Description:**
  The system SHALL validate extracted data against predefined rules for
  the document type (e.g. required fields present, date formats, numeric
  ranges, totals reconciliation).

- **Inputs:**
  - `document_id`
  - Extracted fields

- **Processing:**
  1. Load validation rules for the document type.
  2. Invoke the `validate_extraction` tool.
  3. Record pass/fail per rule with details.
  4. Set processing status to `VALIDATED` or `FAILED_VALIDATION`.

- **Outputs:**
  ```json
  {
    "document_id": "DOC-001",
    "valid": false,
    "violations": [
      { "rule": "total_matches_line_items", "message": "Total does not match sum of line items" }
    ]
  }
  ```

- **Acceptance Criteria:**
  - [ ] All configured rules for the document type are evaluated.
  - [ ] Violations include a machine-readable rule ID and a human-readable message.
  - [ ] Documents failing validation are routed to human review.

### Feature: Confidence Scoring

- **Description:**
  The system SHALL compute an overall confidence score for a document
  from its field-level and classification confidence scores, used to
  decide between automatic processing and human review.

- **Processing:**
  1. Invoke the `calculate_confidence` tool with classification and
     field-level confidence scores.
  2. Compute an aggregate confidence score.
  3. Compare against the configured auto-processing threshold.

- **Outputs:**
  ```json
  { "document_id": "DOC-001", "overall_confidence": 0.94, "threshold": 0.9 }
  ```

- **Acceptance Criteria:**
  - [ ] Aggregate confidence is deterministic given the same inputs.
  - [ ] The auto-processing threshold is configurable per document type.

### Feature: Duplicate Detection

- **Description:**
  The system SHALL detect duplicate documents (e.g. the same invoice
  uploaded twice) where applicable to the document type.

- **Processing:**
  1. Invoke the `check_duplicate` tool using a content hash and/or
     extracted key fields (e.g. vendor + invoice number + amount).
  2. Flag the document as a potential duplicate if a match is found.

- **Outputs:**
  ```json
  { "document_id": "DOC-001", "is_duplicate": true, "duplicate_of": "DOC-000" }
  ```

- **Acceptance Criteria:**
  - [ ] Exact re-uploads of the same file are always flagged.
  - [ ] Near-duplicates based on key fields are flagged for human review, not auto-rejected.

### Feature: Human Review

- **Description:**
  The system SHALL route documents that fail confidence, validation, or
  duplicate checks to a human reviewer, and SHALL allow reviewers to
  approve, reject, or correct extracted data.

- **Processing:**
  1. Create a review task referencing the document and the reason for
     routing.
  2. Present extracted fields and violations to the reviewer.
  3. Accept reviewer decision: `approve`, `reject`, or `correct` (with
     field-level corrections).
  4. Persist the decision and update document status accordingly.

- **Outputs:**
  ```json
  { "document_id": "DOC-001", "review_status": "corrected", "reviewed_by": "user-42" }
  ```

- **Acceptance Criteria:**
  - [ ] Only authorized reviewers can act on a review task.
  - [ ] Corrections are persisted as the authoritative field values.
  - [ ] Review decisions are recorded in the audit log.

### Feature: Agent Tool Execution

- **Description:**
  The AI Agent SHALL execute document-processing steps exclusively
  through tools registered in the tool registry, selecting the next tool
  based on current processing state.

- **Processing:**
  1. Load the current document processing state.
  2. Select the next tool according to the configured workflow for the
     document type.
  3. Execute the tool with bounded timeout.
  4. Record the tool execution (input, output, status, duration).
  5. Repeat until a terminal state is reached or the maximum iteration
     count is exceeded.

- **Acceptance Criteria:**
  - [ ] The agent never invokes a tool outside the registry.
  - [ ] Agent runs terminate at a terminal state or the iteration cap, whichever comes first.
  - [ ] Every tool execution is recorded with a status (`success`, `failed`, `timeout`).

### Feature: Audit Logging

- **Description:**
  The system SHALL record every significant agent action, tool
  execution, and reviewer decision in an auditable, append-only log.

- **Acceptance Criteria:**
  - [ ] Audit entries include actor, action, entity, entity ID, and timestamp.
  - [ ] Audit entries are immutable once written.
  - [ ] Audit history for a document can be retrieved via the API.

### Feature: Asynchronous Processing

- **Description:**
  The system SHALL process documents asynchronously after upload,
  decoupling the upload request from the AI Agent workflow.

- **Acceptance Criteria:**
  - [ ] Upload requests return before agent processing completes.
  - [ ] Processing status can be polled or retrieved via the API.
  - [ ] Failed jobs are retried according to a bounded retry policy.

## External Interface Requirements

- **User interfaces:**
  A web dashboard (Next.js) for uploading documents, viewing processing
  status, and performing human review.
- **APIs / integrations:**
  REST API (see [API design](technical-design/api.md)) consumed by the
  frontend and by external systems submitting documents programmatically.
  Outbound integrations with an LLM provider and an OCR provider.
- **Hardware:**
  None; runs on standard server/container infrastructure.

## Quality Attributes

Traceable to NFRs in [Requirements](REQUIREMENTS.md).

| NFR ID | Attribute | Acceptance Threshold |
| ------ | --------- | --------------------- |
| NFR-1 | Performance | Upload acknowledgement < 2s under normal load |
| NFR-2 | Performance | Standard processing completes < 60s under normal load |
| NFR-3 | Performance | Tool calls have configurable, enforced timeouts |
| NFR-4/5 | Security | AuthN required; access limited to authorized documents |
| NFR-7/8 | Security | Only registered tools executed; high-risk actions require human approval |
| NFR-11/12/13 | Reliability | Bounded retries; idempotent job processing; bounded agent iterations |
| NFR-14 | Availability | >= 99% monthly API availability in production |
| NFR-15 | Scalability | Processing decoupled from synchronous requests via queue/workers |
| NFR-18/19 | Observability | Status/error/latency/agent metrics exposed; every run has a trace ID |
| NFR-20 | Data Integrity | Extracted data validated before being persisted as approved |
| NFR-21/22 | Maintainability | System runs as Docker containers; `docker compose up` reproduces the full local stack (API, worker, Postgres, queue) |

## Traceability Matrix

| Requirement ID | SRS Feature | Test Case ID |
| -------------- | ----------- | ------------- |
| FR-1, FR-2, FR-3, FR-4 | Document Upload | TC-UPLOAD-* |
| FR-5, FR-6 | Document Classification | TC-CLASSIFY-* |
| FR-7 | Structured Extraction | TC-EXTRACT-* |
| FR-8 | Confidence Scoring | TC-CONFIDENCE-* |
| FR-9 | Validation | TC-VALIDATE-* |
| FR-10 | Duplicate Detection | TC-DEDUP-* |
| FR-11, FR-12 | Human Review | TC-REVIEW-* |
| FR-13 | Confidence Scoring / Auto-Processing | TC-AUTOPROC-* |
| FR-14, FR-15, FR-22 | Audit Logging | TC-AUDIT-* |
| FR-16 | External API | TC-API-* |
| FR-17, FR-18, FR-19, FR-20, FR-21 | Agent Tool Execution | TC-AGENT-* |
| FR-23 | Agent Tool Execution (workflow config) | TC-AGENT-WORKFLOW-* |
| FR-24 | RAG retrieval tool | TC-RAG-* |
| FR-26, FR-27 | Asynchronous Processing | TC-ASYNC-* |

---

Next stage: [Architecture](architecture/system-architecture.md)
