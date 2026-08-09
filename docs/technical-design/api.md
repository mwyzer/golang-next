# API Design

Concrete API contracts implementing the [System Architecture](../architecture/system-architecture.md)
and satisfying FR-16 of [Requirements](../REQUIREMENTS.md).

## Conventions

- Base URL: `/api/v1`
- Auth scheme: single shared bearer token in `Authorization: Bearer <API_TOKEN>` — **not** per-user JWT. It authenticates a request as a fixed dev tenant/user (seeded by `db/migrations/000002_seed_dev_data.up.sql`); real per-user authentication is a [PRD Open Question](../PRD.md#open-questions), not yet built
- Versioning strategy: URL-prefixed (`/api/v1`); breaking changes bump the prefix
- Error format:

  ```json
  { "error": { "code": "VALIDATION_FAILED", "message": "..." } }
  ```

## Endpoints

| Method | Path | Description | Auth Required |
| --- | --- | --- | --- |
| GET | `/healthz` | Health check | No |
| POST | `/documents` | Upload a document | Yes |
| GET | `/documents/{id}` | Get document detail, status, extracted fields | Yes |
| GET | `/documents/{id}/agent-runs` | List agent runs for a document, each with its tool executions | Yes |
| GET | `/review-queue` | List documents pending human review | Yes (reviewer) |
| POST | `/documents/{id}/review` | Submit a review decision (approve/reject/correct) | Yes (reviewer) |

The following were designed but are **not implemented**: `GET /documents`
(list/filter), `GET /documents/{id}/status` (redundant with the status
already returned by `GET /documents/{id}`), `GET /agent-runs/{id}`
(only the list-by-document form exists), and `GET /documents/{id}/audit-log`
(audit logs are currently write-only via the API — query the
`audit_logs` table directly against Postgres in the meantime, see the
[README's Docker Compose section](../../README.md#operating-with-docker-compose)).

### `POST /documents`

#### POST /documents — Request

```text
Content-Type: multipart/form-data

file: <binary>
```

No `document_type_hint` field — classification is always automatic
(the agent's `classify_document` tool), there's no client-supplied hint
mechanism.

#### POST /documents — Response 201

```json
{ "document_id": "DOC-001", "status": "UPLOADED" }
```

#### POST /documents — Errors

| Status | Condition |
| --- | --- |
| 400 | Missing `file` field, or unsupported MIME type |
| 401 | Missing/invalid auth token |
| 413 | File exceeds configured size limit |

No `403` — there's no per-tenant authorization check today (single
shared token, fixed dev tenant; see Conventions above).

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
| 404 | Document not found, **or** belongs to a different tenant |

Cross-tenant access returns `404`, not `403` — the lookup is
tenant-scoped at the query level (`GetByID(tenantID, id)`), so a
document outside the caller's tenant looks identical to one that
doesn't exist, rather than confirming it exists via a `403` (NFR-5).

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
{ "document_id": "DOC-001", "review_status": "CORRECTED", "reviewed_by": "user-42" }
```

(`review_status` is uppercase — matches the `review_tasks.status`
column's values: `APPROVED`, `REJECTED`, `CORRECTED`.)

#### POST /documents/{id}/review — Errors

| Status | Condition |
| --- | --- |
| 400 | Invalid decision value or malformed corrections |
| 401 | Missing/invalid auth token |
| 403 | User's role is not `reviewer` or `admin` |
| 404 | Document not found, or no open review task for it |
| 409 | Document is not `PENDING_REVIEW` |

---

See also: [DB](db.md) · [Tools](tools.md)
Next stage: [Implementation](../IMPLEMENTATION.md)
