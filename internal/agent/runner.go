// Package agent implements the AI Agent workflow described in
// docs/architecture/agent-architecture.md. This slice implements only
// the Document Classification feature (SRS): run_ocr -> classify_document,
// then either mark the document CLASSIFIED or route it to human review.
package agent

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"golang-nextjs/internal/db"
	"golang-nextjs/internal/domain"
	"golang-nextjs/internal/providers/llm"
	"golang-nextjs/internal/providers/ocr"
)

const (
	toolRunOCR           = "run_ocr"
	toolClassifyDocument = "classify_document"
	toolExtractFields    = "extract_fields"

	auditActorAgent = "agent"
)

type Runner struct {
	Pool            *pgxpool.Pool
	Documents       *db.DocumentRepo
	DocumentTypes   *db.DocumentTypeRepo
	ExtractedFields *db.ExtractedFieldRepo
	AgentRuns       *db.AgentRunRepo
	ToolExecutions  *db.ToolExecutionRepo
	Reviews         *db.ReviewTaskRepo
	Audit           *db.AuditLogRepo
	OCR             ocr.Provider
	LLM             llm.Provider
	MaxIterations   int
}

// PollOnce claims and fully processes at most one UPLOADED document. It
// returns processed=false when there was nothing to do, so the caller
// can back off before polling again.
func (r *Runner) PollOnce(ctx context.Context) (processed bool, err error) {
	documentID, agentRunID, ok, err := db.ClaimNextUploaded(ctx, r.Pool, r.MaxIterations)
	if err != nil {
		return false, fmt.Errorf("claim next document: %w", err)
	}
	if !ok {
		return false, nil
	}

	r.process(ctx, documentID, agentRunID)
	return true, nil
}

func (r *Runner) process(ctx context.Context, documentID, agentRunID string) {
	iterations := 0

	doc, err := r.Documents.GetByIDAny(ctx, documentID)
	if err != nil {
		log.Printf("agent run %s: load document %s: %v", agentRunID, documentID, err)
		r.fail(ctx, "", documentID, agentRunID, iterations, "load_document_failed")
		return
	}

	// Step 1: run_ocr
	iterations++
	if iterations > r.MaxIterations {
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "max_iterations_exceeded")
		return
	}
	text, err := r.OCR.ExtractText(ctx, doc.FilePath)
	ocrInput := map[string]any{"document_id": documentID, "file_path": doc.FilePath}
	if err != nil {
		_ = r.ToolExecutions.Record(ctx, agentRunID, toolRunOCR, ocrInput,
			map[string]any{"error": err.Error()}, domain.ToolExecutionFailed)
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "ocr_failed")
		return
	}
	_ = r.ToolExecutions.Record(ctx, agentRunID, toolRunOCR, ocrInput,
		map[string]any{"text_length": len(text)}, domain.ToolExecutionSuccess)

	// Step 2: classify_document
	iterations++
	if iterations > r.MaxIterations {
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "max_iterations_exceeded")
		return
	}
	result, err := r.LLM.Classify(ctx, text)
	classifyInput := map[string]any{"document_id": documentID, "text_length": len(text)}
	if err != nil {
		_ = r.ToolExecutions.Record(ctx, agentRunID, toolClassifyDocument, classifyInput,
			map[string]any{"error": err.Error()}, domain.ToolExecutionFailed)
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "classification_failed")
		return
	}
	_ = r.ToolExecutions.Record(ctx, agentRunID, toolClassifyDocument, classifyInput,
		map[string]any{"document_type": result.DocumentType, "confidence": result.Confidence},
		domain.ToolExecutionSuccess)

	if result.DocumentType == domain.DocumentTypeUnknown {
		r.routeToReview(ctx, doc.TenantID, documentID, agentRunID, iterations, result.Confidence)
		return
	}

	if err := r.Documents.MarkClassified(ctx, documentID, result.DocumentType, result.Confidence); err != nil {
		log.Printf("agent run %s: mark classified: %v", agentRunID, err)
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "persist_classification_failed")
		return
	}
	_ = r.Audit.Record(ctx, doc.TenantID, auditActorAgent, "document.classified",
		domain.AuditEntityDocument, documentID,
		map[string]any{"document_type": result.DocumentType, "confidence": result.Confidence, "agent_run_id": agentRunID})

	// Step 3: extract_fields
	iterations++
	if iterations > r.MaxIterations {
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "max_iterations_exceeded")
		return
	}
	schema, err := r.DocumentTypes.GetFieldSchema(ctx, result.DocumentType)
	if err != nil {
		log.Printf("agent run %s: load field schema: %v", agentRunID, err)
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "load_field_schema_failed")
		return
	}

	extraction, err := r.LLM.Extract(ctx, text, schema)
	extractInput := map[string]any{"document_id": documentID, "document_type": result.DocumentType, "schema": schema}
	if err != nil {
		_ = r.ToolExecutions.Record(ctx, agentRunID, toolExtractFields, extractInput,
			map[string]any{"error": err.Error()}, domain.ToolExecutionFailed)
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "extraction_failed")
		return
	}

	fields := make([]domain.ExtractedField, 0, len(extraction.Fields))
	extractOutput := make(map[string]any, len(extraction.Fields))
	missing := 0
	for name, fe := range extraction.Fields {
		fields = append(fields, domain.ExtractedField{
			DocumentID: documentID,
			FieldName:  name,
			Value:      fe.Value,
			Confidence: fe.Confidence,
			Source:     domain.ExtractedFieldSourceExtraction,
		})
		extractOutput[name] = map[string]any{"value": fe.Value, "confidence": fe.Confidence}
		if fe.Value == nil {
			missing++
		}
	}

	if err := r.ExtractedFields.InsertBatch(ctx, documentID, fields); err != nil {
		log.Printf("agent run %s: persist extracted fields: %v", agentRunID, err)
		_ = r.ToolExecutions.Record(ctx, agentRunID, toolExtractFields, extractInput,
			map[string]any{"error": err.Error()}, domain.ToolExecutionFailed)
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "persist_extracted_fields_failed")
		return
	}
	_ = r.ToolExecutions.Record(ctx, agentRunID, toolExtractFields, extractInput,
		extractOutput, domain.ToolExecutionSuccess)

	if err := r.Documents.MarkExtracted(ctx, documentID); err != nil {
		log.Printf("agent run %s: mark extracted: %v", agentRunID, err)
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "persist_extracted_status_failed")
		return
	}

	if err := r.AgentRuns.Finish(ctx, agentRunID, domain.AgentRunCompleted, iterations); err != nil {
		log.Printf("agent run %s: finish: %v", agentRunID, err)
	}
	_ = r.Audit.Record(ctx, doc.TenantID, auditActorAgent, "document.extracted",
		domain.AuditEntityDocument, documentID,
		map[string]any{"field_count": len(fields), "missing_count": missing, "agent_run_id": agentRunID})

	log.Printf("agent run %s: document %s extracted %d fields (%d missing)",
		agentRunID, documentID, len(fields), missing)
}

func (r *Runner) routeToReview(ctx context.Context, tenantID, documentID, agentRunID string, iterations int, confidence float64) {
	if err := r.Reviews.Create(ctx, documentID, domain.ReviewReasonUnknownType); err != nil {
		log.Printf("agent run %s: create review task: %v", agentRunID, err)
	}
	if err := r.Documents.MarkPendingReview(ctx, documentID, &confidence); err != nil {
		log.Printf("agent run %s: mark pending review: %v", agentRunID, err)
	}
	if err := r.AgentRuns.Finish(ctx, agentRunID, domain.AgentRunCompleted, iterations); err != nil {
		log.Printf("agent run %s: finish: %v", agentRunID, err)
	}
	_ = r.Audit.Record(ctx, tenantID, auditActorAgent, "document.routed_to_review",
		domain.AuditEntityDocument, documentID,
		map[string]any{"reason": domain.ReviewReasonUnknownType, "agent_run_id": agentRunID})

	log.Printf("agent run %s: document %s routed to human review (unknown type)", agentRunID, documentID)
}

func (r *Runner) fail(ctx context.Context, tenantID, documentID, agentRunID string, iterations int, reason string) {
	if err := r.Documents.MarkFailed(ctx, documentID); err != nil {
		log.Printf("agent run %s: mark failed: %v", agentRunID, err)
	}
	if err := r.AgentRuns.Finish(ctx, agentRunID, domain.AgentRunFailed, iterations); err != nil {
		log.Printf("agent run %s: finish: %v", agentRunID, err)
	}
	if tenantID != "" {
		_ = r.Audit.Record(ctx, tenantID, auditActorAgent, "agent_run.failed",
			domain.AuditEntityAgentRun, agentRunID,
			map[string]any{"reason": reason, "document_id": documentID})
	}
	log.Printf("agent run %s: document %s failed (%s)", agentRunID, documentID, reason)
}
