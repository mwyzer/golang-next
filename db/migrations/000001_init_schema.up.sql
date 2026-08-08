CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE tenants (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL REFERENCES tenants(id),
    email       text NOT NULL UNIQUE,
    role        text NOT NULL CHECK (role IN ('uploader', 'reviewer', 'admin')),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE document_types (
    id                      text PRIMARY KEY,
    field_schema            jsonb NOT NULL,
    validation_rules        jsonb NOT NULL,
    auto_process_threshold  numeric NOT NULL
);

CREATE TABLE documents (
    id                          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                   uuid NOT NULL REFERENCES tenants(id),
    uploaded_by                 uuid NOT NULL REFERENCES users(id),
    document_type_id            text REFERENCES document_types(id),
    status                      text NOT NULL CHECK (status IN (
        'UPLOADED', 'CLASSIFIED', 'EXTRACTED', 'VALIDATED',
        'PENDING_REVIEW', 'AUTO_PROCESSED', 'REVIEWED', 'FAILED'
    )),
    file_path                   text NOT NULL,
    mime_type                   text NOT NULL,
    file_size_bytes             bigint NOT NULL,
    content_hash                text NOT NULL,
    classification_confidence   numeric,
    overall_confidence          numeric,
    is_duplicate                boolean NOT NULL DEFAULT false,
    duplicate_of_document_id    uuid REFERENCES documents(id),
    created_at                  timestamptz NOT NULL DEFAULT now(),
    updated_at                  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_documents_tenant_status ON documents (tenant_id, status);
CREATE INDEX idx_documents_content_hash ON documents (content_hash);

CREATE TABLE extracted_fields (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id  uuid NOT NULL REFERENCES documents(id),
    field_name   text NOT NULL,
    field_value  jsonb NOT NULL,
    confidence   numeric NOT NULL,
    source       text NOT NULL CHECK (source IN ('extraction', 'review_correction')),
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_extracted_fields_document ON extracted_fields (document_id);

CREATE TABLE agent_runs (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id       uuid NOT NULL REFERENCES documents(id),
    status            text NOT NULL CHECK (status IN ('RUNNING', 'COMPLETED', 'FAILED')),
    iteration_count   int NOT NULL DEFAULT 0,
    max_iterations    int NOT NULL,
    started_at        timestamptz NOT NULL DEFAULT now(),
    finished_at       timestamptz,
    trace_id          text NOT NULL
);

CREATE INDEX idx_agent_runs_document ON agent_runs (document_id);

CREATE TABLE tool_executions (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_run_id   uuid NOT NULL REFERENCES agent_runs(id),
    tool_name      text NOT NULL,
    input          jsonb NOT NULL,
    output         jsonb,
    status         text NOT NULL CHECK (status IN ('SUCCESS', 'FAILED', 'TIMEOUT')),
    started_at     timestamptz NOT NULL DEFAULT now(),
    finished_at    timestamptz
);

CREATE INDEX idx_tool_executions_agent_run ON tool_executions (agent_run_id);

CREATE TABLE review_tasks (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id   uuid NOT NULL REFERENCES documents(id),
    reason        text NOT NULL CHECK (reason IN ('LOW_CONFIDENCE', 'VALIDATION_FAILED', 'DUPLICATE', 'UNKNOWN_TYPE')),
    status        text NOT NULL CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED', 'CORRECTED')),
    assigned_to   uuid REFERENCES users(id),
    reviewed_by   uuid REFERENCES users(id),
    notes         text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    reviewed_at   timestamptz
);

CREATE INDEX idx_review_tasks_status_assigned ON review_tasks (status, assigned_to);

CREATE TABLE audit_logs (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    uuid NOT NULL REFERENCES tenants(id),
    actor        text NOT NULL,
    action       text NOT NULL,
    entity_type  text NOT NULL CHECK (entity_type IN ('document', 'agent_run', 'review_task')),
    entity_id    uuid NOT NULL,
    metadata     jsonb,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_logs_entity ON audit_logs (entity_type, entity_id);

CREATE TABLE knowledge_chunks (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   uuid NOT NULL REFERENCES tenants(id),
    content     text NOT NULL,
    embedding   vector(1536) NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_knowledge_chunks_embedding ON knowledge_chunks USING hnsw (embedding vector_cosine_ops);
