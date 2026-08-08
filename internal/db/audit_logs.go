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
