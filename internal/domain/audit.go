package domain

import "time"

type AuditEntityType string

const (
	AuditEntityDocument   AuditEntityType = "document"
	AuditEntityAgentRun   AuditEntityType = "agent_run"
	AuditEntityReviewTask AuditEntityType = "review_task"
)

// AuditLog is an immutable, append-only record of a significant action
// (NFR-10). Actor is a user ID or the literal "agent".
type AuditLog struct {
	ID         string
	TenantID   string
	Actor      string
	Action     string
	EntityType AuditEntityType
	EntityID   string
	Metadata   map[string]any
	CreatedAt  time.Time
}
