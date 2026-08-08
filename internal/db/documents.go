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

// MarkPendingReview moves a document into human review. confidence is
// optional (e.g. an unknown-classification routing wants to record it;
// a validation-failure routing doesn't have a new value to set) — pass
// nil to leave the existing classification_confidence untouched.
func (r *DocumentRepo) MarkPendingReview(ctx context.Context, id string, confidence *float64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE documents
		SET status = $1,
		    classification_confidence = COALESCE($2, classification_confidence),
		    updated_at = now()
		WHERE id = $3`,
		domain.StatusPendingReview, confidence, id,
	)
	if err != nil {
		return fmt.Errorf("mark document pending review: %w", err)
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
