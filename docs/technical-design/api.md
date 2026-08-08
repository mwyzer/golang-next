# API Design

Concrete API contracts implementing the [System Architecture](../architecture/system-architecture.md)
and satisfying FR-16 of [Requirements](../REQUIREMENTS.md).

## Conventions

- Base URL: `/api/v1`
- Auth scheme: Bearer token (JWT) in `Authorization` header
- Versioning strategy: URL-prefixed (`/api/v1`); breaking changes bump the prefix
- Error format:

  ```json
  { "error": { "code": "VALIDATION_FAILED", "message": "..." } }
  ```

## Endpoints

| Method | Path | Description | Auth Required |
| --- | --- | --- | --- |
| POST | `/documents` | Upload a document | Yes |
| GET | `/documents` | List documents (filter by status/type) | Yes |
| GET | `/documents/{id}` | Get document detail, status, extracted fields | Yes |
| GET | `/documents/{id}/status` | Poll processing status | Yes |
| GET | `/documents/{id}/agent-runs` | List agent runs for a document | Yes |
| GET | `/agent-runs/{id}` | Get agent run detail incl. tool executions | Yes |
| GET | `/review-queue` | List documents pending human review | Yes (reviewer) |
| POST | `/documents/{id}/review` | Submit a review decision (approve/reject/correct) | Yes (reviewer) |
| GET | `/documents/{id}/audit-log` | Get audit history for a document | Yes |

### `POST /documents`

#### POST /documents — Request

```text
Content-Type: multipart/form-data

file: <binary>
document_type_hint: "invoice" (optional)
```

#### POST /documents — Response 201

```json
{ "document_id": "DOC-001", "status": "UPLOADED" }
```

#### POST /documents — Errors

| Status | Condition |
| --- | --- |
| 400 | Unsupported file type/extension |
| 401 | Missing/invalid auth token |
| 403 | Not authorized for target tenant |
| 413 | File exceeds configured size limit |

### `GET /documents/{id}`

#### GET /documents/{id} — Response 200

```json
{
  "document_id": "DOC-001",
  "status": "PENDING_REVIEW",
  "document_type": "invoice",
  "classification_confidence": 0.97,
  "overall_confidence": 0.81,
  "fields": {
    "vendor_name": { "value": "Acme Corp", "confidence": 0.95 },
    "total_amount": { "value": 129.99, "confidence": 0.7 }
  },
  "created_at": "2026-08-09T10:00:00Z"
}
```

#### GET /documents/{id} — Errors

| Status | Condition |
| --- | --- |
| 401 | Missing/invalid auth token |
| 403 | Not authorized for this document |
| 404 | Document not found |

### `POST /documents/{id}/review`

#### POST /documents/{id}/review — Request

```json
{
  "decision": "correct",
  "corrected_fields": { "total_amount": 139.99 },
  "notes": "Total was misread by OCR"
}
```

#### POST /documents/{id}/review — Response 200

```json
{ "document_id": "DOC-001", "review_status": "corrected", "reviewed_by": "user-42" }
```

#### POST /documents/{id}/review — Errors

| Status | Condition |
| --- | --- |
| 400 | Invalid decision value or malformed corrections |
| 401 | Missing/invalid auth token |
| 403 | User is not an authorized reviewer for this tenant |
| 404 | Document or review task not found |
| 409 | Document is not in a reviewable state |

---

See also: [DB](db.md) · [Tools](tools.md)
Next stage: [Implementation](../IMPLEMENTATION.md)
