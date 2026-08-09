package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"golang-nextjs/internal/domain"
)

type AuditLogRepo struct {
	pool *pgxpool.Pool
}

func NewAuditLogRepo(pool *pgxpool.Pool) *AuditLogRepo {
	return &AuditLogRepo{pool: pool}
}

// Record appends an immutable audit entry (NFR-10). There is no update
// or delete method by design.
func (r *AuditLogRepo) Record(ctx context.Context, tenantID, actor, action string, entityType domain.AuditEntityType, entityID string, metadata map[string]any) error {
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO audit_logs (id, tenant_id, actor, action, entity_type, entity_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		uuid.NewString(), tenantID, actor, action, entityType, entityID, metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("record audit log: %w", err)
	}
	return nil
}

// ListByDocument returns the full audit trail for a document, oldest
// first (SRS Feature: Audit Logging — "audit history for a document
// can be retrieved via the API"). A document's history is spread
// across three entity types (document.*, agent_run.*, review.*
// actions), so this joins agent_runs and review_tasks to pull in every
// audit_logs row tied to any of them, not just rows keyed directly by
// the document ID.
func (r *AuditLogRepo) ListByDocument(ctx context.Context, tenantID, documentID string) ([]domain.AuditLog, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT al.id, al.tenant_id, al.actor, al.action, al.entity_type, al.entity_id, al.metadata, al.created_at
		FROM audit_logs al
		WHERE al.tenant_id = $1 AND (
			(al.entity_type = 'document' AND al.entity_id = $2)
			OR (al.entity_type = 'agent_run' AND al.entity_id IN (
				SELECT id FROM agent_runs WHERE document_id = $2
			))
			OR (al.entity_type = 'review_task' AND al.entity_id IN (
				SELECT id FROM review_tasks WHERE document_id = $2
			))
		)
		ORDER BY al.created_at ASC`, tenantID, documentID)
	if err != nil {
		return nil, fmt.Errorf("list audit log for document: %w", err)
	}
	defer rows.Close()

	var logs []domain.AuditLog
	for rows.Next() {
		var (
			l            domain.AuditLog
			metadataJSON []byte
		)
		if err := rows.Scan(&l.ID, &l.TenantID, &l.Actor, &l.Action, &l.EntityType, &l.EntityID, &metadataJSON, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &l.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal audit metadata: %w", err)
			}
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
