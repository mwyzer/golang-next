// Agent worker entrypoint. Polls for UPLOADED documents and runs the
// agent workflow (docs/architecture/agent-architecture.md) against
// them. See internal/agent for the workflow itself.
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"golang-nextjs/internal/agent"
	"golang-nextjs/internal/config"
	"golang-nextjs/internal/db"
	"golang-nextjs/internal/providers/llm"
	"golang-nextjs/internal/providers/ocr"
	"golang-nextjs/internal/storage"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	store, err := storage.NewLocalStore(cfg.StorageRoot)
	if err != nil {
		log.Fatalf("init storage: %v", err)
	}

	runner := &agent.Runner{
		Pool:           pool,
		Documents:      db.NewDocumentRepo(pool),
		AgentRuns:      db.NewAgentRunRepo(pool),
		ToolExecutions: db.NewToolExecutionRepo(pool),
		Reviews:        db.NewReviewTaskRepo(pool),
		Audit:          db.NewAuditLogRepo(pool),
		OCR:            ocr.StubProvider{Store: store},
		LLM:            llm.StubProvider{},
		MaxIterations:  int(cfg.MaxAgentIterations),
	}

	pollInterval := time.Duration(cfg.PollIntervalSecs) * time.Second
	log.Printf("worker started, polling every %s", pollInterval)

	for {
		select {
		case <-ctx.Done():
			log.Println("worker shutting down")
			return
		default:
		}

		processed, err := runner.PollOnce(ctx)
		if err != nil {
			log.Printf("poll error: %v", err)
		}
		if !processed {
			select {
			case <-ctx.Done():
				log.Println("worker shutting down")
				return
			case <-time.After(pollInterval):
			}
		}
	}
}
