package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"golang-nextjs/internal/db"
	"golang-nextjs/internal/domain"
)

type ReviewHandler struct {
	Documents       *db.DocumentRepo
	Reviews         *db.ReviewTaskRepo
	ExtractedFields *db.ExtractedFieldRepo
	Audit           *db.AuditLogRepo
}

type reviewRequest struct {
	Decision        string         `json:"decision"`
	CorrectedFields map[string]any `json:"corrected_fields"`
	Notes           string         `json:"notes"`
}

var decisionToReviewStatus = map[string]domain.ReviewStatus{
	"approve": domain.ReviewStatusApproved,
	"reject":  domain.ReviewStatusRejected,
	"correct": domain.ReviewStatusCorrected,
}

var decisionToAuditAction = map[string]string{
	"approve": "review.approved",
	"reject":  "review.rejected",
	"correct": "review.corrected",
}

// Submit implements POST /documents/{id}/review (docs/technical-design/api.md,
// SRS Feature: Human Review). Reviewer-only — see RequireRole.
func (h *ReviewHandler) Submit(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantIDFromContext(r.Context())
	userID := userIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "could not parse request body: "+err.Error())
		return
	}

	reviewStatus, ok := decisionToReviewStatus[req.Decision]
	if !ok {
		writeError(w, http.StatusBadRequest, "INVALID_DECISION", `decision must be one of "approve", "reject", "correct"`)
		return
	}
	if req.Decision == "correct" && len(req.CorrectedFields) == 0 {
		writeError(w, http.StatusBadRequest, "MISSING_CORRECTIONS", `decision "correct" requires a non-empty corrected_fields`)
		return
	}

	doc, err := h.Documents.GetByID(r.Context(), tenantID, id)
	if errors.Is(err, domain.ErrDocumentNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "document not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "failed to load document")
		return
	}
	if doc.Status != domain.StatusPendingReview {
		writeError(w, http.StatusConflict, "NOT_REVIEWABLE", "document is not in a reviewable state")
		return
	}

	task, err := h.Reviews.GetPendingByDocument(r.Context(), id)
	if errors.Is(err, db.ErrReviewTaskNotFound) {
		writeError(w, http.StatusConflict, "NOT_REVIEWABLE", "no open review task for this document")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "failed to load review task")
		return
	}

	if req.Decision == "correct" {
		fields := make([]domain.ExtractedField, 0, len(req.CorrectedFields))
		for name, value := range req.CorrectedFields {
			fields = append(fields, domain.ExtractedField{
				DocumentID: id,
				FieldName:  name,
				Value:      value,
				Confidence: 1.0,
				Source:     domain.ExtractedFieldSourceReviewCorrection,
			})
		}
		if err := h.ExtractedFields.InsertBatch(r.Context(), id, fields); err != nil {
			writeError(w, http.StatusInternalServerError, "DB_ERROR", "failed to save corrected fields")
			return
		}
	}

	if err := h.Reviews.Resolve(r.Context(), task.ID, reviewStatus, userID, req.Notes); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "failed to resolve review task")
		return
	}
	if err := h.Documents.MarkReviewed(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "failed to update document status")
		return
	}

	auditMetadata := map[string]any{"decision": req.Decision, "document_id": id}
	if req.Decision == "correct" {
		auditMetadata["corrected_fields"] = req.CorrectedFields
	}
	if req.Notes != "" {
		auditMetadata["notes"] = req.Notes
	}
	_ = h.Audit.Record(r.Context(), tenantID, userID, decisionToAuditAction[req.Decision],
		domain.AuditEntityReviewTask, task.ID, auditMetadata)

	writeJSON(w, http.StatusOK, map[string]any{
		"document_id":   id,
		"review_status": reviewStatus,
		"reviewed_by":   userID,
	})
}

// Queue implements GET /review-queue. Reviewer-only — see RequireRole.
func (h *ReviewHandler) Queue(w http.ResponseWriter, r *http.Request) {
	tenantID := tenantIDFromContext(r.Context())

	items, err := h.Reviews.ListPendingByTenant(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", "failed to load review queue")
		return
	}

	type queueItemView struct {
		ReviewTaskID             string              `json:"review_task_id"`
		DocumentID               string              `json:"document_id"`
		DocumentType             *string             `json:"document_type"`
		Reason                   domain.ReviewReason `json:"reason"`
		ClassificationConfidence *float64            `json:"classification_confidence"`
		OverallConfidence        *float64            `json:"overall_confidence"`
		CreatedAt                string              `json:"created_at"`
	}

	views := make([]queueItemView, 0, len(items))
	for _, item := range items {
		views = append(views, queueItemView{
			ReviewTaskID:             item.ReviewTaskID,
			DocumentID:               item.DocumentID,
			DocumentType:             item.DocumentType,
			Reason:                   item.Reason,
			ClassificationConfidence: item.ClassificationConfidence,
			OverallConfidence:        item.OverallConfidence,
			CreatedAt:                item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"review_queue": views})
}
