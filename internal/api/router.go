package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"golang-nextjs/internal/db"
	"golang-nextjs/internal/storage"
)

type Deps struct {
	Repo          *db.DocumentRepo
	AgentRuns     *db.AgentRunRepo
	ToolExecs     *db.ToolExecutionRepo
	Store         storage.Store
	MaxUploadSize int64
	APIToken      string
}

func NewRouter(deps Deps) http.Handler {
	docs := &DocumentsHandler{
		Repo:          deps.Repo,
		AgentRuns:     deps.AgentRuns,
		ToolExecs:     deps.ToolExecs,
		Store:         deps.Store,
		MaxUploadSize: deps.MaxUploadSize,
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
	})

	return r
}
