package main

import (
	"context"
	"log"
	"net/http"

	"golang-nextjs/internal/api"
	"golang-nextjs/internal/config"
	"golang-nextjs/internal/db"
	"golang-nextjs/internal/storage"
)

func main() {
	ctx := context.Background()
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

	router := api.NewRouter(api.Deps{
		Repo:            db.NewDocumentRepo(pool),
		AgentRuns:       db.NewAgentRunRepo(pool),
		ToolExecs:       db.NewToolExecutionRepo(pool),
		ExtractedFields: db.NewExtractedFieldRepo(pool),
		Reviews:         db.NewReviewTaskRepo(pool),
		Users:           db.NewUserRepo(pool),
		Audit:           db.NewAuditLogRepo(pool),
		Store:           store,
		MaxUploadSize:   cfg.MaxUploadBytes,
		APIToken:        cfg.APIToken,
	})

	log.Printf("api listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
