package domain

import "time"

type AgentRunStatus string

const (
	AgentRunRunning   AgentRunStatus = "RUNNING"
	AgentRunCompleted AgentRunStatus = "COMPLETED"
	AgentRunFailed    AgentRunStatus = "FAILED"
)

// AgentRun is a single execution of the agent workflow for a document
// (docs/architecture/agent-architecture.md).
type AgentRun struct {
	ID             string
	DocumentID     string
	Status         AgentRunStatus
	IterationCount int
	MaxIterations  int
	StartedAt      time.Time
	FinishedAt     *time.Time
	TraceID        string
}

type ToolExecutionStatus string

const (
	ToolExecutionSuccess ToolExecutionStatus = "SUCCESS"
	ToolExecutionFailed  ToolExecutionStatus = "FAILED"
	ToolExecutionTimeout ToolExecutionStatus = "TIMEOUT"
)

// ToolExecution is a single tool invocation within an agent run. Only
// tools in the registry described in agent-architecture.md may appear
// here (NFR-7).
type ToolExecution struct {
	ID         string              `json:"id"`
	AgentRunID string              `json:"agent_run_id"`
	ToolName   string              `json:"tool_name"`
	Input      map[string]any      `json:"input"`
	Output     map[string]any      `json:"output"`
	Status     ToolExecutionStatus `json:"status"`
	StartedAt  time.Time           `json:"started_at"`
	FinishedAt *time.Time          `json:"finished_at"`
}

type ReviewReason string

const (
	ReviewReasonLowConfidence    ReviewReason = "LOW_CONFIDENCE"
	ReviewReasonValidationFailed ReviewReason = "VALIDATION_FAILED"
	ReviewReasonDuplicate        ReviewReason = "DUPLICATE"
	ReviewReasonUnknownType      ReviewReason = "UNKNOWN_TYPE"
)

type ReviewStatus string

const (
	ReviewStatusPending   ReviewStatus = "PENDING"
	ReviewStatusApproved  ReviewStatus = "APPROVED"
	ReviewStatusRejected  ReviewStatus = "REJECTED"
	ReviewStatusCorrected ReviewStatus = "CORRECTED"
)

// ReviewTask is a human review request for a document (SRS Feature:
// Human Review).
type ReviewTask struct {
	ID         string
	DocumentID string
	Reason     ReviewReason
	Status     ReviewStatus
	CreatedAt  time.Time
}
