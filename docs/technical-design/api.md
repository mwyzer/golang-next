# API Design

Concrete API contracts implementing the [System Architecture](../architecture/system-architecture.md)
and satisfying FR-16 of [Requirements](../REQUIREMENTS.md).

## Conventions

- Base URL: `/api/v1`
- Auth scheme: per-user bearer token in `Authorization: Bearer <token>`
  — **not** JWT. Each user has their own opaque, randomly generated
  token (`internal/auth.GenerateToken`); only its SHA-256 hash is
  persisted (`users.token_hash`), so a database leak doesn't directly
  expose usable credentials. `RequireAuth` looks up the token's hash
  and injects the matching user's tenant ID, user ID, and role into
  context — each caller is authenticated as themselves, not as a fixed
  shared identity. There's still only one tenant in practice (the
  seeded dev tenant); multi-tenancy remains a [PRD Open Question](../PRD.md#open-questions).
  New users are provisioned via `POST /users` (admin-only) or by
  bootstrapping the seeded dev user's token from `API_TOKEN` on API
  startup (`cmd/api/main.go`) — there's no self-service signup/login.
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
| GET | `/documents/{id}/audit-log` | Get audit history for a document (spans `document.*`, `agent_run.*`, and `review.*` entries tied to it) | Yes |
| GET | `/review-queue` | List documents pending human review | Yes (reviewer) |
| POST | `/documents/{id}/review` | Submit a review decision (approve/reject/correct) | Yes (reviewer) |
| POST | `/users` | Provision a new user and issue their bearer token | Yes (admin) |

The following were designed but are **not implemented**: `GET /documents`
(list/filter) and `GET /agent-runs/{id}` (only the list-by-document
form exists). `GET /documents/{id}/status` was also dropped from the
design — it would have been redundant with the status `GET /documents/{id}`
already returns.

### `POST /users`

#### POST /users — Request

```json
{ "email": "reviewer@example.com", "role": "reviewer" }
```

`role` must be one of `uploader`, `reviewer`, `admin`.

#### POST /users — Response 201

```json
{
  "user_id": "USR-001",
  "email": "reviewer@example.com",
  "role": "reviewer",
  "token": "3f9c2a1b..."
}
```

`token` is shown **exactly once**, in this response — only its hash is
persisted, so it can't be recovered later. If it's lost, the only fix
is issuing the user a new one (there's no rotate-existing-user
endpoint yet, only creation of new users).

#### POST /users — Errors

| Status | Condition |
| --- | --- |
| 400 | Missing email, or `role` isn't one of `uploader`/`reviewer`/`admin` |
| 401 | Missing/invalid auth token |
| 403 | Caller's role isn't `admin` |
| 409 | A user with this email already exists |

### `GET /documents/{id}/audit-log`

#### GET /documents/{id}/audit-log — Response 200

```json
{
  "audit_log": [
    {
      "actor": "agent",
      "action": "document.classified",
      "entity_type": "document",
      "entity_id": "DOC-001",
      "metadata": { "document_type": "invoice", "confidence": 0.95 },
      "created_at": "2026-08-09T10:00:05Z"
    },
    {
      "actor": "user-42",
      "action": "review.approved",
      "entity_type": "review_task",
      "entity_id": "RT-001",
      "metadata": { "decision": "approve" },
      "created_at": "2026-08-09T10:05:00Z"
    }
  ]
}
```

Ordered oldest first. `entity_id` differs per row — `document` entries
are keyed by the document's own ID, but `agent_run`/`review_task`
entries are keyed by that agent run's or review task's ID, since a
single document can have several of each over its lifetime.

#### GET /documents/{id}/audit-log — Errors

| Status | Condition |
| --- | --- |
| 401 | Missing/invalid auth token |
| 404 | Document not found, or belongs to a different tenant |

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

No `403` — there's no per-tenant authorization check today. Auth is
per-user, but there's still only one tenant in practice (see
Conventions above), so there's no second tenant to enforce a boundary
against yet.

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
