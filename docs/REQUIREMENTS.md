# Requirements

Derived from the [PRD](PRD.md). Split into functional and non-functional
requirements, each with a unique ID for traceability into the [SRS](SRS.md).

## Functional Requirements

| ID | Requirement | Priority | Source |
| ---- | ----------- | -------- | -------- |
| FR-1 | The system SHALL allow authenticated users to upload supported documents. | Must | PRD Goals |
| FR-2 | The system SHALL validate file type and file size before accepting a document. | Must | PRD Constraints |
| FR-3 | The system SHALL assign a unique ID to every uploaded document. | Must | PRD Goals |
| FR-4 | The system SHALL store the original uploaded document. | Must | PRD Goals |
| FR-5 | The system SHALL classify documents into supported document types. | Must | PRD Goals |
| FR-6 | The system SHALL support invoice, receipt, and CV document types in the MVP. | Must | PRD Assumptions |
| FR-7 | The system SHALL extract structured information according to the document type schema. | Must | PRD Goals |
| FR-8 | The system SHALL provide field-level confidence scores for extracted information. | Must | PRD Goals |
| FR-9 | The system SHALL validate extracted data against predefined validation rules. | Must | PRD Goals |
| FR-10 | The system SHALL detect duplicate documents where applicable. | Must | PRD Goals |
| FR-11 | The system SHALL route documents below the configured confidence threshold to human review. | Must | PRD Goals |
| FR-12 | The system SHALL allow reviewers to approve, reject, or correct extracted information. | Must | PRD Target Users |
| FR-13 | The system SHALL automatically process documents that satisfy configured confidence and validation rules. | Must | PRD Goals |
| FR-14 | The system SHALL maintain processing status for every document. | Must | PRD Goals |
| FR-15 | The system SHALL maintain a history of document processing activities. | Must | PRD Goals |
| FR-16 | The system SHALL provide an API for uploading, retrieving, processing, and reviewing documents. | Must | PRD Goals |
| FR-17 | The system SHALL execute document-processing actions through registered tools. | Must | PRD Agentic AI |
| FR-18 | The AI Agent SHALL be able to select an appropriate tool based on the current processing state. | Must | PRD Agentic AI |
| FR-19 | The AI Agent SHALL be able to execute multiple tools sequentially within a single document-processing workflow. | Must | PRD Agentic AI |
| FR-20 | The AI Agent SHALL stop processing when the workflow reaches a terminal state. | Must | PRD Agentic AI |
| FR-21 | The system SHALL enforce a maximum number of agent iterations per processing run. | Must | PRD Security |
| FR-22 | The system SHALL record every AI Agent run and tool execution. | Must | PRD Goals |
| FR-23 | The system SHALL support configurable document-processing workflows by document type. | Should | PRD Goals |
| FR-24 | The system SHOULD support retrieval of company policies or knowledge through RAG. | Should | PRD Goals |
| FR-25 | The system SHOULD allow administrators to configure confidence thresholds. | Should | PRD Target Users |
| FR-26 | The system SHOULD support asynchronous document processing. | Should | PRD Constraints |
| FR-27 | The system SHOULD support retrying failed processing steps when the failure is recoverable. | Should | PRD Agentic AI |
| FR-28 | The system MAY support additional document types after the MVP. | Could | PRD Non-Goals |

## Non-Functional Requirements

| ID | Requirement | Priority | Notes |
| ----- | ----------- | -------- | ----- |
| NFR-1 | Performance: API requests for document upload SHALL return an acknowledgement within 2 seconds under normal load. | Must | Does not include asynchronous AI processing time. |
| NFR-2 | Performance: The system SHOULD complete standard document processing within 60 seconds under normal load. | Should | Processing time depends on OCR and LLM providers. |
| NFR-3 | Performance: Agent tool calls SHALL have configurable timeouts. | Must | Prevent indefinitely running tools. |
| NFR-4 | Security: Only authenticated users SHALL access protected document operations. | Must | Authentication required. |
| NFR-5 | Security: Users SHALL only access documents they are authorized to access. | Must | Enforce authorization at API/service layer. |
| NFR-6 | Security: Uploaded files SHALL be validated before processing. | Must | File type and size validation. |
| NFR-7 | Security: AI Agents SHALL only execute tools registered in the tool registry. | Must | No arbitrary tool execution. |
| NFR-8 | Security: High-risk actions SHALL require explicit human approval. | Must | Human-in-the-loop. |
| NFR-9 | Security: Secrets and API credentials SHALL NOT be stored in source code. | Must | Environment variables or secret manager. |
| NFR-10 | Security: All significant agent actions SHALL be recorded in audit logs. | Must | Required for traceability. |
| NFR-11 | Reliability: Recoverable processing failures SHALL support retry. | Must | Retry policy must be bounded. |
| NFR-12 | Reliability: The system SHALL prevent duplicate processing of the same job. | Must | Idempotency required. |
| NFR-13 | Reliability: Agent execution SHALL terminate after the configured maximum number of iterations. | Must | Prevent infinite agent loops. |
| NFR-14 | Availability: The API SHOULD provide at least 99% monthly availability in production. | Should | Excludes planned maintenance. |
| NFR-15 | Scalability: Document processing SHALL be decoupled from synchronous API requests. | Should | Use background workers/queue. |
| NFR-16 | Maintainability: The backend SHALL follow idiomatic Go project structure and conventions. | Must | Go-first architecture. |
| NFR-17 | Maintainability: LLM, OCR, and embedding providers SHALL be abstracted behind provider interfaces. | Should | Enables provider replacement. |
| NFR-18 | Observability: The system SHALL expose processing status, errors, latency, and agent execution metrics. | Must | Required for production debugging. |
| NFR-19 | Observability: Each document-processing run SHALL have a traceable execution ID. | Must | Correlates agent/tool activity. |
| NFR-20 | Data Integrity: Extracted data SHALL be validated before being persisted as approved data. | Must | Prevent invalid records. |
| NFR-21 | Maintainability: The system SHALL be containerized using Docker. | Must | PRD Constraints. |
| NFR-22 | Maintainability: Local development SHALL be reproducible using Docker Compose. | Must | Covers API server, agent worker, Postgres, and queue. |

## Out of Scope

- Training a custom OCR model.
- Training a custom LLM.
- Fully autonomous processing of high-risk documents.
- Supporting every possible document format.
- Full enterprise document management.
- Full ERP/accounting integration in the MVP.
- Multi-tenant SaaS architecture in the MVP.
- Automatic execution of destructive operations without human approval.
- Arbitrary code execution by the AI Agent.
- Building a custom vector database.
- Real-time collaborative document editing.

---

Next stage: [SRS](SRS.md)