package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

// GetRole returns a user's role (e.g. "uploader", "reviewer", "admin"),
// used to gate reviewer-only endpoints (NFR-5, SRS Feature: Human Review).
func (r *UserRepo) GetRole(ctx context.Context, userID string) (string, error) {
	var role string
	err := r.pool.QueryRow(ctx, `SELECT role FROM users WHERE id = $1`, userID).Scan(&role)
	if err != nil {
		return "", fmt.Errorf("get user role: %w", err)
	}
	return role, nil
}
