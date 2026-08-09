package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

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
	// CORSAllowedOrigins lets the web UI (served from a different
	// origin/port than the API) call it from a browser at all —
	// without this, every request the browser makes is blocked by the
	// same-origin policy before it ever reaches RequireAuth.
	CORSAllowedOrigins []string
}

// reviewerRoles gates POST /documents/{id}/review and GET /review-queue
// (SRS Feature: Human Review — "only authorized reviewers").
var reviewerRoles = []string{"reviewer", "admin"}

// adminRoles gates user provisioning — only an admin can issue new
// bearer tokens.
var adminRoles = []string{"admin"}

func NewRouter(deps Deps) http.Handler {
	docs := &DocumentsHandler{
		Repo:            deps.Repo,
		AgentRuns:       deps.AgentRuns,
		ToolExecs:       deps.ToolExecs,
		ExtractedFields: deps.ExtractedFields,
		Audit:           deps.Audit,
		Store:           deps.Store,
		MaxUploadSize:   deps.MaxUploadSize,
	}
	reviews := &ReviewHandler{
		Documents:       deps.Repo,
		Reviews:         deps.Reviews,
		ExtractedFields: deps.ExtractedFields,
		Audit:           deps.Audit,
	}
	users := &UsersHandler{Users: deps.Users}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   deps.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false, // auth is a bearer token, not a cookie
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(RequireAuth(deps.Users))
		r.Post("/documents", docs.Upload)
		r.Get("/documents/{id}", docs.Get)
		r.Get("/documents/{id}/agent-runs", docs.ListAgentRuns)
		r.Get("/documents/{id}/audit-log", docs.ListAuditLog)

		r.Group(func(r chi.Router) {
			r.Use(RequireRole(reviewerRoles...))
			r.Post("/documents/{id}/review", reviews.Submit)
			r.Get("/review-queue", reviews.Queue)
		})

		r.Group(func(r chi.Router) {
			r.Use(RequireRole(adminRoles...))
			r.Post("/users", users.Create)
		})
	})

	return r
}
