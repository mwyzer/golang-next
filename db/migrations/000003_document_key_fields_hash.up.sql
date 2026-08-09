ALTER TABLE documents ADD COLUMN key_fields_hash text;

-- Partial index: most documents never get a key_fields_hash written
-- (extraction incomplete, or the pipeline never reached check_duplicate),
-- so only index the rows near-duplicate detection actually queries.
CREATE INDEX idx_documents_key_fields_hash
    ON documents (tenant_id, document_type_id, key_fields_hash)
    WHERE key_fields_hash IS NOT NULL;
