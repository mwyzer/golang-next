package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"golang-nextjs/internal/auth"
	"golang-nextjs/internal/domain"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrEmailTaken   = errors.New("email already in use")
)

// pgUniqueViolation is Postgres' SQLSTATE for a unique constraint violation.
const pgUniqueViolation = "23505"

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// GetByTokenHash looks up the user a bearer token belongs to, by the
// token's SHA-256 hash (internal/auth.HashToken) — tokens are never
// stored in plaintext. Used by RequireAuth on every authenticated
// request.
func (r *UserRepo) GetByTokenHash(ctx context.Context, tokenHash string) (domain.User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, tenant_id, email, role FROM users WHERE token_hash = $1`, tokenHash)

	var u domain.User
	err := row.Scan(&u.ID, &u.TenantID, &u.Email, &u.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user by token: %w", err)
	}
	return u, nil
}

// SetTokenHash assigns or rotates a user's bearer-token hash. Used to
// bootstrap the seeded dev user's token from API_TOKEN on startup
// (cmd/api/main.go).
func (r *UserRepo) SetTokenHash(ctx context.Context, userID, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET token_hash = $1 WHERE id = $2`, tokenHash, userID)
	if err != nil {
		return fmt.Errorf("set user token hash: %w", err)
	}
	return nil
}

// Create adds a new user with a freshly generated bearer token,
// returning the plaintext token — the only time it's ever available,
// since only its hash is persisted (SRS Feature: Human Review — "only
// authorized reviewers can act", now backed by real per-user identity
// rather than a single token shared by every caller).
func (r *UserRepo) Create(ctx context.Context, tenantID, email, role string) (domain.User, string, error) {
	token, tokenHash, err := auth.GenerateToken()
	if err != nil {
		return domain.User{}, "", fmt.Errorf("generate token: %w", err)
	}

	id := uuid.NewString()
	_, err = r.pool.Exec(ctx, `
		INSERT INTO users (id, tenant_id, email, role, token_hash)
		VALUES ($1, $2, $3, $4, $5)`,
		id, tenantID, email, role, tokenHash,
	)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return domain.User{}, "", ErrEmailTaken
	}
	if err != nil {
		return domain.User{}, "", fmt.Errorf("create user: %w", err)
	}

	return domain.User{ID: id, TenantID: tenantID, Email: email, Role: role}, token, nil
}
