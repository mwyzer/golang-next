//go:build integration || e2e

// Package testutil provides a Postgres testcontainer shared across all
// tests in a single test binary (one per package: internal/db,
// internal/agent, internal/api, e2e). Starting the container is slow,
// so TestMain calls StartShared once and every test resets just the
// mutable tables via Reset instead of paying startup cost per test.
package testutil

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"golang-nextjs/internal/db"
)

// pgImage must match docker-compose.yml's postgres service: the schema
// migration creates the `vector` extension for knowledge_chunks, which
// plain postgres images don't ship.
const pgImage = "pgvector/pgvector:pg16"

// activityTables are truncated by Reset between tests. tenants, users,
// and document_types are seeded reference data (db/migrations/000002)
// and are left intact so tests can rely on the fixed dev tenant/user/
// document-type IDs being present.
var activityTables = []string{
	"tool_executions",
	"audit_logs",
	"review_tasks",
	"extracted_fields",
	"agent_runs",
	"documents",
	"knowledge_chunks",
}

var (
	container *postgres.PostgresContainer
	pool      *pgxpool.Pool
)

// StartShared starts the shared container and applies migrations. Call
// once per test binary, from TestMain.
func StartShared(ctx context.Context) (*pgxpool.Pool, error) {
	c, err := postgres.Run(ctx, pgImage,
		postgres.WithDatabase("docagent"),
		postgres.WithUsername("docagent"),
		postgres.WithPassword("docagent"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}
	container = c

	connStr, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("get connection string: %w", err)
	}

	p, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := p.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	if err := db.Migrate(ctx, p); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	pool = p
	return pool, nil
}

// StopShared closes the pool and terminates the container. Call once
// per test binary, from TestMain after m.Run().
func StopShared(ctx context.Context) {
	if pool != nil {
		pool.Close()
	}
	if container != nil {
		_ = container.Terminate(ctx)
	}
}

// Pool returns the pool started by StartShared.
func Pool() *pgxpool.Pool {
	return pool
}

// Reset truncates every activity table so each test starts from the
// seeded baseline (fixed dev tenant/user, three document_types) with no
// leftover documents/runs/reviews from a previous test.
func Reset(ctx context.Context, pool *pgxpool.Pool) error {
	stmt := "TRUNCATE TABLE "
	for i, t := range activityTables {
		if i > 0 {
			stmt += ", "
		}
		stmt += t
	}
	stmt += " RESTART IDENTITY CASCADE"

	if _, err := pool.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("truncate activity tables: %w", err)
	}
	return nil
}
