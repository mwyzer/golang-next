package main

import (
	"context"
	"log"
	"net/http"

	"golang-nextjs/internal/api"
	"golang-nextjs/internal/auth"
	"golang-nextjs/internal/config"
	"golang-nextjs/internal/db"
	"golang-nextjs/internal/storage"
)

// devUserID matches the seeded row in
// db/migrations/000002_seed_dev_data.up.sql. Auth is per-user
// (internal/api/middleware.go), but the seed migration can't read
// API_TOKEN itself, so this bootstraps the dev user's token from it on
// every startup — local dev keeps working with a fixed, configurable
// token without needing a real signup flow.
const devUserID = "00000000-0000-0000-0000-000000000002"

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

	users := db.NewUserRepo(pool)
	if cfg.APIToken != "" {
		if err := users.SetTokenHash(ctx, devUserID, auth.HashToken(cfg.APIToken)); err != nil {
			log.Fatalf("bootstrap dev user token: %v", err)
		}
	}

	store, err := storage.NewLocalStore(cfg.StorageRoot)
	if err != nil {
		log.Fatalf("init storage: %v", err)
	}

	router := api.NewRouter(api.Deps{
		Repo:               db.NewDocumentRepo(pool),
		AgentRuns:          db.NewAgentRunRepo(pool),
		ToolExecs:          db.NewToolExecutionRepo(pool),
		ExtractedFields:    db.NewExtractedFieldRepo(pool),
		Reviews:            db.NewReviewTaskRepo(pool),
		Users:              users,
		Audit:              db.NewAuditLogRepo(pool),
		Store:              store,
		MaxUploadSize:      cfg.MaxUploadBytes,
		CORSAllowedOrigins: cfg.CORSAllowedOrigins,
	})

	log.Printf("api listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
