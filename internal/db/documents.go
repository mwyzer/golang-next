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
