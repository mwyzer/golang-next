DROP INDEX IF EXISTS idx_users_token_hash;
ALTER TABLE users DROP COLUMN IF EXISTS token_hash;
