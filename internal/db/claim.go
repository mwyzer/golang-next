package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"golang-nextjs/internal/domain"
)

// ClaimNextUploaded atomically picks the oldest UPLOADED document that
// has no in-flight agent run and opens a new RUNNING agent run for it,
// so multiple worker instances never process the same document twice
// (docs/architecture/agent-architecture.md).
func ClaimNextUploaded(ctx context.Context, pool *pgxpool.Pool, maxIterations int) (documentID string, agentRunID string, ok bool, err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", "", false, fmt.Errorf("begin claim tx: %w", err)
	}
	defer tx.Rollback(ctx)

	row := tx.QueryRow(ctx, `
		SELECT d.id
		FROM documents d
		WHERE d.status = $1
		  AND NOT EXISTS (
		      SELECT 1 FROM agent_runs ar
		      WHERE ar.document_id = d.id AND ar.status = $2
		  )
		ORDER BY d.created_at ASC
		LIMIT 1
		FOR UPDATE OF d SKIP LOCKED`,
		domain.StatusUploaded, domain.AgentRunRunning,
	)

	if err := row.Scan(&documentID); errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	} else if err != nil {
		return "", "", false, fmt.Errorf("claim next document: %w", err)
	}

	agentRunID = uuid.NewString()
	traceID := uuid.NewString()

	if _, err := tx.Exec(ctx, `
		INSERT INTO agent_runs (id, document_id, status, max_iterations, trace_id)
		VALUES ($1, $2, $3, $4, $5)`,
		agentRunID, documentID, domain.AgentRunRunning, maxIterations, traceID,
	); err != nil {
		return "", "", false, fmt.Errorf("open agent run: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", "", false, fmt.Errorf("commit claim tx: %w", err)
	}

	return documentID, agentRunID, true, nil
}
