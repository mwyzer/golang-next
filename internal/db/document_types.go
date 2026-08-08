package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DocumentTypeRepo struct {
	pool *pgxpool.Pool
}

func NewDocumentTypeRepo(pool *pgxpool.Pool) *DocumentTypeRepo {
	return &DocumentTypeRepo{pool: pool}
}

// GetFieldSchema returns the field name -> type map for a document type
// (e.g. {"vendor_name": "string", "total_amount": "number"}), used by
// the extract_fields tool.
func (r *DocumentTypeRepo) GetFieldSchema(ctx context.Context, documentTypeID string) (map[string]string, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx,
		`SELECT field_schema FROM document_types WHERE id = $1`, documentTypeID,
	).Scan(&raw)
	if err != nil {
		return nil, fmt.Errorf("get field schema: %w", err)
	}

	var schema map[string]string
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("unmarshal field schema: %w", err)
	}
	return schema, nil
}
