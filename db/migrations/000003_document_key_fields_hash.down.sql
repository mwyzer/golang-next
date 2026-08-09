DROP INDEX IF EXISTS idx_documents_key_fields_hash;
ALTER TABLE documents DROP COLUMN IF EXISTS key_fields_hash;
