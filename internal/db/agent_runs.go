package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"golang-nextjs/internal/domain"
)

type AgentRunRepo struct {
	pool *pgxpool.Pool
}

func NewAgentRunRepo(pool *pgxpool.Pool) *AgentRunRepo {
	return &AgentRunRepo{pool: pool}
}

// Finish records the terminal state of an agent run opened by ClaimNextUploaded.
func (r *AgentRunRepo) Finish(ctx context.Context, id string, status domain.AgentRunStatus, iterationCount int) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE agent_runs
		SET status = $1, iteration_count = $2, finished_at = now()
		WHERE id = $3`,
		status, iterationCount, id,
	)
	if err != nil {
		return fmt.Errorf("finish agent run: %w", err)
	}
	return nil
}

// CountFailed returns how many agent runs for a document have finished
// FAILED, used to decide whether a failure is still within the bounded
// retry budget (FR-27, NFR-11: "recoverable processing failures SHALL
// support retry... bounded").
func (r *AgentRunRepo) CountFailed(ctx context.Context, documentID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_runs WHERE document_id = $1 AND status = $2`,
		documentID, domain.AgentRunFailed,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count failed agent runs: %w", err)
	}
	return count, nil
}

// ListByDocument returns agent runs for a document, most recent first.
func (r *AgentRunRepo) ListByDocument(ctx context.Context, documentID string) ([]domain.AgentRun, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, document_id, status, iteration_count, max_iterations,
		       started_at, finished_at, trace_id
		FROM agent_runs
		WHERE document_id = $1
		ORDER BY started_at DESC`, documentID)
	if err != nil {
		return nil, fmt.Errorf("list agent runs: %w", err)
	}
	defer rows.Close()

	var runs []domain.AgentRun
	for rows.Next() {
		var run domain.AgentRun
		if err := rows.Scan(
			&run.ID, &run.DocumentID, &run.Status, &run.IterationCount, &run.MaxIterations,
			&run.StartedAt, &run.FinishedAt, &run.TraceID,
		); err != nil {
			return nil, fmt.Errorf("scan agent run: %w", err)
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}
