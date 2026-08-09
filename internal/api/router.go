package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"golang-nextjs/internal/db"
	"golang-nextjs/internal/storage"
)

type Deps struct {
	Repo            *db.DocumentRepo
	AgentRuns       *db.AgentRunRepo
	ToolExecs       *db.ToolExecutionRepo
	ExtractedFields *db.ExtractedFieldRepo
	Reviews         *db.ReviewTaskRepo
	Users           *db.UserRepo
	Audit           *db.AuditLogRepo
	Store           storage.Store
	MaxUploadSize   int64
	APIToken        string
}

// reviewerRoles gates POST /documents/{id}/review and GET /review-queue
// (SRS Feature: Human Review — "only authorized reviewers").
var reviewerRoles = []string{"reviewer", "admin"}

func NewRouter(deps Deps) http.Handler {
	docs := &DocumentsHandler{
		Repo:            deps.Repo,
		AgentRuns:       deps.AgentRuns,
		ToolExecs:       deps.ToolExecs,
		ExtractedFields: deps.ExtractedFields,
		Store:           deps.Store,
		MaxUploadSize:   deps.MaxUploadSize,
	}
	reviews := &ReviewHandler{
		Documents:       deps.Repo,
		Reviews:         deps.Reviews,
		ExtractedFields: deps.ExtractedFields,
		Audit:           deps.Audit,
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(RequireAuth(deps.APIToken))
		r.Post("/documents", docs.Upload)
		r.Get("/documents/{id}", docs.Get)
		r.Get("/documents/{id}/agent-runs", docs.ListAgentRuns)

		r.Group(func(r chi.Router) {
			r.Use(RequireRole(deps.Users, reviewerRoles...))
			r.Post("/documents/{id}/review", reviews.Submit)
			r.Get("/review-queue", reviews.Queue)
		})
	})

	return r
}
