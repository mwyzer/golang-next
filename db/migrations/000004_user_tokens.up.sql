ALTER TABLE users ADD COLUMN token_hash text;

-- Unique so two users can never collide on the same token hash; NULL
-- is allowed (a user has no token until one is issued/rotated) and
-- multiple NULLs don't violate uniqueness in Postgres.
CREATE UNIQUE INDEX idx_users_token_hash ON users (token_hash) WHERE token_hash IS NOT NULL;
