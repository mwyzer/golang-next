package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"golang-nextjs/internal/domain"
)

var ErrReviewTaskNotFound = errors.New("review task not found")

// ReviewQueueItem is a review task joined with the document summary a
// reviewer needs to triage it, for GET /review-queue.
type ReviewQueueItem struct {
	ReviewTaskID             string
	DocumentID               string
	DocumentType             *string
	Reason                   domain.ReviewReason
	ClassificationConfidence *float64
	OverallConfidence        *float64
	CreatedAt                time.Time
}

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

// GetPendingByDocument returns the open review task for a document —
// there should be at most one, since a document only re-enters
// PENDING_REVIEW after its previous task is resolved.
func (r *ReviewTaskRepo) GetPendingByDocument(ctx context.Context, documentID string) (domain.ReviewTask, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, document_id, reason, status, created_at
		FROM review_tasks
		WHERE document_id = $1 AND status = $2
		ORDER BY created_at DESC
		LIMIT 1`, documentID, domain.ReviewStatusPending)

	var t domain.ReviewTask
	err := row.Scan(&t.ID, &t.DocumentID, &t.Reason, &t.Status, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ReviewTask{}, ErrReviewTaskNotFound
	}
	if err != nil {
		return domain.ReviewTask{}, fmt.Errorf("get pending review task: %w", err)
	}
	return t, nil
}

// Resolve records a reviewer's decision on a review task (SRS Feature:
// Human Review, "review decisions are recorded").
func (r *ReviewTaskRepo) Resolve(ctx context.Context, id string, status domain.ReviewStatus, reviewedBy, notes string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE review_tasks
		SET status = $1, reviewed_by = $2, notes = NULLIF($3, ''), reviewed_at = now()
		WHERE id = $4`,
		status, reviewedBy, notes, id,
	)
	if err != nil {
		return fmt.Errorf("resolve review task: %w", err)
	}
	return nil
}

// ListPendingByTenant returns the review queue: every open task for
// documents in the tenant, oldest first.
func (r *ReviewTaskRepo) ListPendingByTenant(ctx context.Context, tenantID string) ([]ReviewQueueItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT rt.id, rt.document_id, d.document_type_id, rt.reason,
		       d.classification_confidence, d.overall_confidence, rt.created_at
		FROM review_tasks rt
		JOIN documents d ON d.id = rt.document_id
		WHERE d.tenant_id = $1 AND rt.status = $2
		ORDER BY rt.created_at ASC`, tenantID, domain.ReviewStatusPending)
	if err != nil {
		return nil, fmt.Errorf("list review queue: %w", err)
	}
	defer rows.Close()

	var items []ReviewQueueItem
	for rows.Next() {
		var item ReviewQueueItem
		if err := rows.Scan(
			&item.ReviewTaskID, &item.DocumentID, &item.DocumentType, &item.Reason,
			&item.ClassificationConfidence, &item.OverallConfidence, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan review queue item: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
