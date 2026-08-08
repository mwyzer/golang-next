package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"golang-nextjs/internal/domain"
)

type ReviewTaskRepo struct {
	pool *pgxpool.Pool
}

func NewReviewTaskRepo(pool *pgxpool.Pool) *ReviewTaskRepo {
	return &ReviewTaskRepo{pool: pool}
}

// Create opens a new pending review task for a document (SRS Feature: Human Review).
func (r *ReviewTaskRepo) Create(ctx context.Context, documentID string, reason domain.ReviewReason) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO review_tasks (id, document_id, reason, status)
		VALUES ($1, $2, $3, $4)`,
		uuid.NewString(), documentID, reason, domain.ReviewStatusPending,
	)
	if err != nil {
		return fmt.Errorf("create review task: %w", err)
	}
	return nil
}
