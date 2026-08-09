package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"golang-nextjs/internal/domain"
)

type DocumentRepo struct {
	pool *pgxpool.Pool
}

func NewDocumentRepo(pool *pgxpool.Pool) *DocumentRepo {
	return &DocumentRepo{pool: pool}
}

func (r *DocumentRepo) Create(ctx context.Context, d domain.Document) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO documents (
			id, tenant_id, uploaded_by, status, file_path,
			mime_type, file_size_bytes, content_hash
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		d.ID, d.TenantID, d.UploadedBy, d.Status, d.FilePath,
		d.MimeType, d.FileSizeBytes, d.ContentHash,
	)
	if err != nil {
		return fmt.Errorf("insert document: %w", err)
	}
	return nil
}

func (r *DocumentRepo) GetByID(ctx context.Context, tenantID, id string) (domain.Document, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, uploaded_by, document_type_id, status, file_path,
		       mime_type, file_size_bytes, content_hash, classification_confidence,
		       overall_confidence, is_duplicate, duplicate_of_document_id,
		       created_at, updated_at
		FROM documents
		WHERE id = $1 AND tenant_id = $2`, id, tenantID)

	var d domain.Document
	err := row.Scan(
		&d.ID, &d.TenantID, &d.UploadedBy, &d.DocumentTypeID, &d.Status, &d.FilePath,
		&d.MimeType, &d.FileSizeBytes, &d.ContentHash, &d.ClassificationConfidence,
		&d.OverallConfidence, &d.IsDuplicate, &d.DuplicateOfDocumentID,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Document{}, domain.ErrDocumentNotFound
	}
	if err != nil {
		return domain.Document{}, fmt.Errorf("get document: %w", err)
	}
	return d, nil
}

// GetByIDAny fetches a document without a tenant filter. It's for
// internal worker use only (the agent runs across tenants); API
// handlers must use GetByID so cross-tenant access stays impossible
// (NFR-5).
func (r *DocumentRepo) GetByIDAny(ctx context.Context, id string) (domain.Document, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, uploaded_by, document_type_id, status, file_path,
		       mime_type, file_size_bytes, content_hash, classification_confidence,
		       overall_confidence, is_duplicate, duplicate_of_document_id,
		       created_at, updated_at
		FROM documents
		WHERE id = $1`, id)

	var d domain.Document
	err := row.Scan(
		&d.ID, &d.TenantID, &d.UploadedBy, &d.DocumentTypeID, &d.Status, &d.FilePath,
		&d.MimeType, &d.FileSizeBytes, &d.ContentHash, &d.ClassificationConfidence,
		&d.OverallConfidence, &d.IsDuplicate, &d.DuplicateOfDocumentID,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Document{}, domain.ErrDocumentNotFound
	}
	if err != nil {
		return domain.Document{}, fmt.Errorf("get document: %w", err)
	}
	return d, nil
}

// MarkClassified persists a successful classification result (SRS
// Feature: Document Classification).
func (r *DocumentRepo) MarkClassified(ctx context.Context, id, documentTypeID string, confidence float64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE documents
		SET document_type_id = $1, classification_confidence = $2,
		    status = $3, updated_at = now()
		WHERE id = $4`,
		documentTypeID, confidence, domain.StatusClassified, id,
	)
	if err != nil {
		return fmt.Errorf("mark document classified: %w", err)
	}
	return nil
}

// MarkExtracted records that structured extraction completed (SRS
// Feature: Structured Extraction).
func (r *DocumentRepo) MarkExtracted(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE documents
		SET status = $1, updated_at = now()
		WHERE id = $2`,
		domain.StatusExtracted, id,
	)
	if err != nil {
		return fmt.Errorf("mark document extracted: %w", err)
	}
	return nil
}

// MarkValidated records that a document passed validation (SRS Feature: Validation).
func (r *DocumentRepo) MarkValidated(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE documents
		SET status = $1, updated_at = now()
		WHERE id = $2`,
		domain.StatusValidated, id,
	)
	if err != nil {
		return fmt.Errorf("mark document validated: %w", err)
	}
	return nil
}

// MarkPendingReview moves a document into human review. Both confidence
// values are optional and independent: classificationConfidence is set
// by the unknown-type routing path, overallConfidence by the
// low-confidence routing path; validation-failure and duplicate routing
// pass nil for both. A nil leaves the existing column untouched — they
// are never used to blank out a value the document already has.
func (r *DocumentRepo) MarkPendingReview(ctx context.Context, id string, classificationConfidence, overallConfidence *float64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE documents
		SET status = $1,
		    classification_confidence = COALESCE($2, classification_confidence),
		    overall_confidence = COALESCE($3, overall_confidence),
		    updated_at = now()
		WHERE id = $4`,
		domain.StatusPendingReview, classificationConfidence, overallConfidence, id,
	)
	if err != nil {
		return fmt.Errorf("mark document pending review: %w", err)
	}
	return nil
}

// MarkDuplicateOf flags a document as a duplicate of an earlier one
// (SRS Feature: Duplicate Detection, FR-10). It does not change status —
// callers route the document to review separately.
func (r *DocumentRepo) MarkDuplicateOf(ctx context.Context, id, duplicateOfID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE documents
		SET is_duplicate = true, duplicate_of_document_id = $1, updated_at = now()
		WHERE id = $2`,
		duplicateOfID, id,
	)
	if err != nil {
		return fmt.Errorf("mark document duplicate: %w", err)
	}
	return nil
}

// MarkAutoProcessed records that a document cleared every gate
// (classification, validation, duplicate check, confidence threshold)
// and was finalized without human review (finalize_document tool).
func (r *DocumentRepo) MarkAutoProcessed(ctx context.Context, id string, overallConfidence float64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE documents
		SET status = $1, overall_confidence = $2, updated_at = now()
		WHERE id = $3`,
		domain.StatusAutoProcessed, overallConfidence, id,
	)
	if err != nil {
		return fmt.Errorf("mark document auto processed: %w", err)
	}
	return nil
}

// MarkReviewed records that a human reviewer resolved a PENDING_REVIEW
// document — the terminal status for the human-reviewed path, whatever
// the decision (approve/reject/correct); the decision itself lives on
// the review_tasks row (SRS Feature: Human Review).
func (r *DocumentRepo) MarkReviewed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE documents
		SET status = $1, updated_at = now()
		WHERE id = $2`,
		domain.StatusReviewed, id,
	)
	if err != nil {
		return fmt.Errorf("mark document reviewed: %w", err)
	}
	return nil
}

// MarkFailed records that agent processing could not complete (FR-20).
func (r *DocumentRepo) MarkFailed(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE documents
		SET status = $1, updated_at = now()
		WHERE id = $2`,
		domain.StatusFailed, id,
	)
	if err != nil {
		return fmt.Errorf("mark document failed: %w", err)
	}
	return nil
}

// FindByContentHash returns the first document with a matching content
// hash for the tenant, used for exact-duplicate detection (FR-10).
func (r *DocumentRepo) FindByContentHash(ctx context.Context, tenantID, hash string) (domain.Document, bool, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, uploaded_by, document_type_id, status, file_path,
		       mime_type, file_size_bytes, content_hash, classification_confidence,
		       overall_confidence, is_duplicate, duplicate_of_document_id,
		       created_at, updated_at
		FROM documents
		WHERE tenant_id = $1 AND content_hash = $2
		ORDER BY created_at ASC
		LIMIT 1`, tenantID, hash)

	var d domain.Document
	err := row.Scan(
		&d.ID, &d.TenantID, &d.UploadedBy, &d.DocumentTypeID, &d.Status, &d.FilePath,
		&d.MimeType, &d.FileSizeBytes, &d.ContentHash, &d.ClassificationConfidence,
		&d.OverallConfidence, &d.IsDuplicate, &d.DuplicateOfDocumentID,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Document{}, false, nil
	}
	if err != nil {
		return domain.Document{}, false, fmt.Errorf("find document by hash: %w", err)
	}
	return d, true, nil
}
