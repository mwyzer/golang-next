package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"golang-nextjs/internal/domain"
)

type ExtractedFieldRepo struct {
	pool *pgxpool.Pool
}

func NewExtractedFieldRepo(pool *pgxpool.Pool) *ExtractedFieldRepo {
	return &ExtractedFieldRepo{pool: pool}
}

// InsertBatch persists every field from a single extraction pass,
// including ones with a nil Value (schema expected them but the
// extractor didn't find them) so they stay visible rather than silently
// dropped (SRS Feature: Structured Extraction).
func (r *ExtractedFieldRepo) InsertBatch(ctx context.Context, documentID string, fields []domain.ExtractedField) error {
	if len(fields) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin insert extracted fields: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, f := range fields {
		valueJSON, err := json.Marshal(f.Value)
		if err != nil {
			return fmt.Errorf("marshal field %q value: %w", f.FieldName, err)
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO extracted_fields (id, document_id, field_name, field_value, confidence, source)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			uuid.NewString(), documentID, f.FieldName, valueJSON, f.Confidence, f.Source,
		); err != nil {
			return fmt.Errorf("insert field %q: %w", f.FieldName, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit insert extracted fields: %w", err)
	}
	return nil
}

// ListByDocument returns the current value of every extracted field for
// a document — the most recent row per field name, so a reviewer
// correction (source=review_correction) shadows the original
// extraction rather than appearing as a second entry. Both rows stay in
// the table; nothing is overwritten or deleted.
func (r *ExtractedFieldRepo) ListByDocument(ctx context.Context, documentID string) ([]domain.ExtractedField, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (field_name)
		       id, document_id, field_name, field_value, confidence, source, created_at
		FROM extracted_fields
		WHERE document_id = $1
		ORDER BY field_name ASC, created_at DESC`, documentID)
	if err != nil {
		return nil, fmt.Errorf("list extracted fields: %w", err)
	}
	defer rows.Close()

	var fields []domain.ExtractedField
	for rows.Next() {
		var (
			f         domain.ExtractedField
			valueJSON []byte
		)
		if err := rows.Scan(&f.ID, &f.DocumentID, &f.FieldName, &valueJSON, &f.Confidence, &f.Source, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan extracted field: %w", err)
		}
		if err := json.Unmarshal(valueJSON, &f.Value); err != nil {
			return nil, fmt.Errorf("unmarshal field %q value: %w", f.FieldName, err)
		}
		fields = append(fields, f)
	}
	return fields, rows.Err()
}
