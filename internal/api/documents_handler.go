package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"golang-nextjs/internal/db"
	"golang-nextjs/internal/domain"
	"golang-nextjs/internal/storage"
)

type DocumentsHandler struct {
	Repo            *db.DocumentRepo
	AgentRuns       *db.AgentRunRepo
	ToolExecs       *db.ToolExecutionRepo
	ExtractedFields *db.ExtractedFieldRepo
	Store           storage.Store
	MaxUploadSize   int64
}

// Upload implements POST /documents (docs/technical-design/api.md,
// SRS Feature: Document Upload / FR-1..FR-4).
func (h *DocumentsHandler) Upload(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantIDFromContext(r.Context())
	userID := userIDFromContext(r.Context())

	// Cap the request body so an oversized upload fails fast instead of
	// buffering an unbounded amount of data (NFR-6).
	r.Body = http.MaxBytesReader(w, r.Body, h.MaxUploadSize+1<<20)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_UPLOAD", "could not parse multipart form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "MISSING_FILE", "form field \"file\" is required")
		return
	}
	defer file.Close()

	mimeType := detectMimeType(file, header)

	if err := ValidateUpload(mimeType, header.Size, h.MaxUploadSize); err != nil {
		switch {
		case errors.Is(err, domain.ErrUnsupportedFileType):
			writeError(w, http.StatusBadRequest, "UNSUPPORTED_FILE_TYPE", "file type "+mimeType+" is not supported")
		case errors.Is(err, domain.ErrFileTooLarge):
			writeError(w, http.StatusRequestEntityTooLarge, "FILE_TOO_LARGE", "file exceeds the maximum allowed size")
		default:
			writeError(w, http.StatusBadRequest, "INVALID_UPLOAD", err.Error())
		}
		return
	}

	docID := uuid.NewString()
	hasher := sha256.New()

	path, err := h.Store.Save(r.Context(), docID, io.TeeReader(file, hasher))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STORAGE_ERROR", "failed to store document")
		return
	}

	doc := domain.Document{
		ID:            docID,
		TenantID:      tenantID,
		UploadedBy:    userID,
		Status:        domain.StatusUploaded,
		FilePath:      path,
		MimeType:      mimeType,
		FileSizeBytes: header.Size,
		ContentHash:   hex.EncodeToString(hasher.Sum(nil)),
	}

	if err := h.Repo.Create(r.Context(), doc); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "failed to save document record")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"document_id": doc.ID,
		"status":      string(doc.Status),
	})
}

// Get implements GET /documents/{id}.
func (h *DocumentsHandler) Get(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	doc, err := h.Repo.GetByID(r.Context(), tenantID, id)
	if errors.Is(err, domain.ErrDocumentNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "document not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "failed to load document")
		return
	}

	extractedFields, err := h.ExtractedFields.ListByDocument(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "failed to load extracted fields")
		return
	}

	fields := make(map[string]any, len(extractedFields))
	for _, f := range extractedFields {
		fields[f.FieldName] = map[string]any{"value": f.Value, "confidence": f.Confidence}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"document_id":               doc.ID,
		"status":                    doc.Status,
		"document_type":             doc.DocumentTypeID,
		"classification_confidence": doc.ClassificationConfidence,
		"overall_confidence":        doc.OverallConfidence,
		"fields":                    fields,
		"created_at":                doc.CreatedAt,
	})
}

// ListAgentRuns implements GET /documents/{id}/agent-runs.
func (h *DocumentsHandler) ListAgentRuns(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	// Confirm the document exists and belongs to this tenant before
	// exposing its agent runs (NFR-5).
	if _, err := h.Repo.GetByID(r.Context(), tenantID, id); errors.Is(err, domain.ErrDocumentNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "document not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "failed to load document")
		return
	}

	runs, err := h.AgentRuns.ListByDocument(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "failed to load agent runs")
		return
	}

	type runView struct {
		AgentRunID     string                 `json:"agent_run_id"`
		Status         domain.AgentRunStatus  `json:"status"`
		IterationCount int                    `json:"iteration_count"`
		MaxIterations  int                    `json:"max_iterations"`
		TraceID        string                 `json:"trace_id"`
		StartedAt      string                 `json:"started_at"`
		ToolExecutions []domain.ToolExecution `json:"tool_executions"`
	}

	views := make([]runView, 0, len(runs))
	for _, run := range runs {
		execs, err := h.ToolExecs.ListByAgentRun(r.Context(), run.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "DB_ERROR", "failed to load tool executions")
			return
		}
		views = append(views, runView{
			AgentRunID:     run.ID,
			Status:         run.Status,
			IterationCount: run.IterationCount,
			MaxIterations:  run.MaxIterations,
			TraceID:        run.TraceID,
			StartedAt:      run.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
			ToolExecutions: execs,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"agent_runs": views})
}

// detectMimeType prefers the client-supplied Content-Type but falls back
// to sniffing the first 512 bytes if it's missing or generic.
func detectMimeType(file multipart.File, header *multipart.FileHeader) string {
	if ct := header.Header.Get("Content-Type"); ct != "" && ct != "application/octet-stream" {
		return ct
	}

	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	_, _ = file.Seek(0, io.SeekStart)
	return http.DetectContentType(buf[:n])
}
