package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"golang-nextjs/internal/domain"
)

type ToolExecutionRepo struct {
	pool *pgxpool.Pool
}

func NewToolExecutionRepo(pool *pgxpool.Pool) *ToolExecutionRepo {
	return &ToolExecutionRepo{pool: pool}
}

// Record stores a completed tool invocation. Tools are expected to run
// to completion (success or failure) before calling Record — there is
// no separate "start" call, keeping the write path a single round trip.
func (r *ToolExecutionRepo) Record(ctx context.Context, agentRunID, toolName string, input, output map[string]any, status domain.ToolExecutionStatus) error {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal tool input: %w", err)
	}
	outputJSON, err := json.Marshal(output)
	if err != nil {
		return fmt.Errorf("marshal tool output: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO tool_executions (id, agent_run_id, tool_name, input, output, status, finished_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())`,
		uuid.NewString(), agentRunID, toolName, inputJSON, outputJSON, status,
	)
	if err != nil {
		return fmt.Errorf("record tool execution: %w", err)
	}
	return nil
}

// ListByAgentRun returns tool executions for an agent run, in execution order.
func (r *ToolExecutionRepo) ListByAgentRun(ctx context.Context, agentRunID string) ([]domain.ToolExecution, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, agent_run_id, tool_name, input, output, status, started_at, finished_at
		FROM tool_executions
		WHERE agent_run_id = $1
		ORDER BY started_at ASC`, agentRunID)
	if err != nil {
		return nil, fmt.Errorf("list tool executions: %w", err)
	}
	defer rows.Close()

	var execs []domain.ToolExecution
	for rows.Next() {
		var (
			te                    domain.ToolExecution
			inputJSON, outputJSON []byte
		)
		if err := rows.Scan(
			&te.ID, &te.AgentRunID, &te.ToolName, &inputJSON, &outputJSON,
			&te.Status, &te.StartedAt, &te.FinishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan tool execution: %w", err)
		}
		if len(inputJSON) > 0 {
			if err := json.Unmarshal(inputJSON, &te.Input); err != nil {
				return nil, fmt.Errorf("unmarshal tool input: %w", err)
			}
		}
		if len(outputJSON) > 0 {
			if err := json.Unmarshal(outputJSON, &te.Output); err != nil {
				return nil, fmt.Errorf("unmarshal tool output: %w", err)
			}
		}
		execs = append(execs, te)
	}
	return execs, rows.Err()
}
