// Agent worker entrypoint. Currently a placeholder: it connects to the
// database so the container is viable in docker-compose, but does not
// yet consume a job queue or run the agent loop described in
// docs/architecture/agent-architecture.md. That lands with the
// Document Classification / Structured Extraction features.
package main

import (
	"context"
	"log"

	"golang-nextjs/internal/config"
	"golang-nextjs/internal/db"
)

func main() {
	ctx := context.Background()
	cfg := config.Load()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pool.Close()

	log.Println("worker connected to database; job queue consumer not yet implemented")
	select {}
}
