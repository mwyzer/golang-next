// Package agent implements the AI Agent workflow described in
// docs/architecture/agent-architecture.md: run_ocr -> classify_document
// -> extract_fields -> validate_extraction -> check_duplicate ->
// calculate_confidence -> finalize_document, routing to human review at
// the first gate that fails (unknown type, validation, duplicate, or
// confidence below the document type's auto-process threshold).
package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"golang-nextjs/internal/db"
	"golang-nextjs/internal/domain"
	"golang-nextjs/internal/providers/llm"
	"golang-nextjs/internal/providers/ocr"
	"golang-nextjs/internal/validation"
)

const (
	toolRunOCR              = "run_ocr"
	toolClassifyDocument    = "classify_document"
	toolExtractFields       = "extract_fields"
	toolValidateExtraction  = "validate_extraction"
	toolCheckDuplicate      = "check_duplicate"
	toolCalculateConfidence = "calculate_confidence"
	toolFinalizeDocument    = "finalize_document"

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
	// ToolTimeout bounds each call to an external provider (OCR, LLM).
	// Zero disables the timeout (NFR-3: "Agent tool calls SHALL have
	// configurable timeouts" — configurable includes "off").
	ToolTimeout time.Duration
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
	toolCtx, cancel := r.withToolTimeout(ctx)
	text, err := r.OCR.ExtractText(toolCtx, doc.FilePath)
	cancel()
	ocrInput := map[string]any{"document_id": documentID, "file_path": doc.FilePath}
	if err != nil {
		status, reason := toolFailure(err, "ocr_failed", "ocr_timeout")
		_ = r.ToolExecutions.Record(ctx, agentRunID, toolRunOCR, ocrInput,
			map[string]any{"error": err.Error()}, status)
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, reason)
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
	toolCtx, cancel = r.withToolTimeout(ctx)
	result, err := r.LLM.Classify(toolCtx, text)
	cancel()
	classifyInput := map[string]any{"document_id": documentID, "text_length": len(text)}
	if err != nil {
		status, reason := toolFailure(err, "classification_failed", "classification_timeout")
		_ = r.ToolExecutions.Record(ctx, agentRunID, toolClassifyDocument, classifyInput,
			map[string]any{"error": err.Error()}, status)
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, reason)
		return
	}
	_ = r.ToolExecutions.Record(ctx, agentRunID, toolClassifyDocument, classifyInput,
		map[string]any{"document_type": result.DocumentType, "confidence": result.Confidence},
		domain.ToolExecutionSuccess)

	if result.DocumentType == domain.DocumentTypeUnknown {
		classificationConfidence := result.Confidence
		r.routeToReview(ctx, doc.TenantID, documentID, agentRunID, iterations,
			domain.ReviewReasonUnknownType, &classificationConfidence, nil, map[string]any{})
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

	toolCtx, cancel = r.withToolTimeout(ctx)
	extraction, err := r.LLM.Extract(toolCtx, text, schema)
	cancel()
	extractInput := map[string]any{"document_id": documentID, "document_type": result.DocumentType, "schema": schema}
	if err != nil {
		status, reason := toolFailure(err, "extraction_failed", "extraction_timeout")
		_ = r.ToolExecutions.Record(ctx, agentRunID, toolExtractFields, extractInput,
			map[string]any{"error": err.Error()}, status)
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, reason)
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
	_ = r.Audit.Record(ctx, doc.TenantID, auditActorAgent, "document.extracted",
		domain.AuditEntityDocument, documentID,
		map[string]any{"field_count": len(fields), "missing_count": missing, "agent_run_id": agentRunID})

	// Only a complete extraction (no missing fields) is trustworthy
	// enough to use for near-duplicate matching in step 5 — see
	// keyFieldsHash.
	if hash, ok := keyFieldsHash(extraction.Fields); ok {
		_ = r.Documents.SetKeyFieldsHash(ctx, documentID, hash)
	}

	// Step 4: validate_extraction
	iterations++
	if iterations > r.MaxIterations {
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "max_iterations_exceeded")
		return
	}
	values := make(map[string]any, len(extraction.Fields))
	for name, fe := range extraction.Fields {
		values[name] = fe.Value
	}
	violations := validation.Validate(schema, values)
	validateInput := map[string]any{"document_id": documentID, "document_type": result.DocumentType}
	_ = r.ToolExecutions.Record(ctx, agentRunID, toolValidateExtraction, validateInput,
		map[string]any{"valid": len(violations) == 0, "violations": violations}, domain.ToolExecutionSuccess)

	if len(violations) > 0 {
		r.routeToReview(ctx, doc.TenantID, documentID, agentRunID, iterations,
			domain.ReviewReasonValidationFailed, nil, nil, map[string]any{"violations": violations})
		return
	}

	if err := r.Documents.MarkValidated(ctx, documentID); err != nil {
		log.Printf("agent run %s: mark validated: %v", agentRunID, err)
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "persist_validated_status_failed")
		return
	}
	_ = r.Audit.Record(ctx, doc.TenantID, auditActorAgent, "document.validated",
		domain.AuditEntityDocument, documentID,
		map[string]any{"agent_run_id": agentRunID})

	// Step 5: check_duplicate
	iterations++
	if iterations > r.MaxIterations {
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "max_iterations_exceeded")
		return
	}
	checkDupInput := map[string]any{"document_id": documentID, "content_hash": doc.ContentHash}

	earliest, foundAny, err := r.Documents.FindByContentHash(ctx, doc.TenantID, doc.ContentHash)
	if err != nil {
		_ = r.ToolExecutions.Record(ctx, agentRunID, toolCheckDuplicate, checkDupInput,
			map[string]any{"error": err.Error()}, domain.ToolExecutionFailed)
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "duplicate_check_failed")
		return
	}
	matchType := "exact_content_hash"
	isDuplicate := foundAny && earliest.ID != documentID

	// An exact content-hash match already settles it; only fall back to
	// key-fields matching (e.g. the same invoice rescanned, different
	// file bytes but identical vendor/amount/date) when that misses.
	if !isDuplicate {
		if hash, ok := keyFieldsHash(extraction.Fields); ok {
			nearMatch, foundNear, err := r.Documents.FindByKeyFieldsHash(ctx, doc.TenantID, result.DocumentType, hash)
			if err != nil {
				_ = r.ToolExecutions.Record(ctx, agentRunID, toolCheckDuplicate, checkDupInput,
					map[string]any{"error": err.Error()}, domain.ToolExecutionFailed)
				r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "duplicate_check_failed")
				return
			}
			if foundNear && nearMatch.ID != documentID {
				earliest = nearMatch
				isDuplicate = true
				matchType = "near_duplicate_key_fields"
			}
		}
	}

	checkDupOutput := map[string]any{"is_duplicate": isDuplicate}
	if isDuplicate {
		checkDupOutput["duplicate_of"] = earliest.ID
		checkDupOutput["match_type"] = matchType
	}
	_ = r.ToolExecutions.Record(ctx, agentRunID, toolCheckDuplicate, checkDupInput, checkDupOutput, domain.ToolExecutionSuccess)

	if isDuplicate {
		if err := r.Documents.MarkDuplicateOf(ctx, documentID, earliest.ID); err != nil {
			log.Printf("agent run %s: mark duplicate: %v", agentRunID, err)
		}
		r.routeToReview(ctx, doc.TenantID, documentID, agentRunID, iterations,
			domain.ReviewReasonDuplicate, nil, nil,
			map[string]any{"duplicate_of": earliest.ID, "match_type": matchType})
		return
	}

	// Step 6: calculate_confidence
	iterations++
	if iterations > r.MaxIterations {
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "max_iterations_exceeded")
		return
	}
	threshold, err := r.DocumentTypes.GetAutoProcessThreshold(ctx, result.DocumentType)
	if err != nil {
		log.Printf("agent run %s: load auto-process threshold: %v", agentRunID, err)
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "load_threshold_failed")
		return
	}

	confidenceSum := result.Confidence
	confidenceCount := 1
	for _, fe := range extraction.Fields {
		confidenceSum += fe.Confidence
		confidenceCount++
	}
	overallConfidence := confidenceSum / float64(confidenceCount)

	calcInput := map[string]any{
		"document_id":               documentID,
		"classification_confidence": result.Confidence,
		"field_count":               len(extraction.Fields),
	}
	_ = r.ToolExecutions.Record(ctx, agentRunID, toolCalculateConfidence, calcInput,
		map[string]any{"overall_confidence": overallConfidence, "threshold": threshold}, domain.ToolExecutionSuccess)

	if overallConfidence < threshold {
		lowConfidence := overallConfidence
		r.routeToReview(ctx, doc.TenantID, documentID, agentRunID, iterations,
			domain.ReviewReasonLowConfidence, nil, &lowConfidence,
			map[string]any{"overall_confidence": overallConfidence, "threshold": threshold})
		return
	}

	// Step 7: finalize_document (high-risk: only reached once validation,
	// duplicate, and confidence gates all pass — see agent-architecture.md)
	iterations++
	if iterations > r.MaxIterations {
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "max_iterations_exceeded")
		return
	}
	if err := r.Documents.MarkAutoProcessed(ctx, documentID, overallConfidence); err != nil {
		log.Printf("agent run %s: mark auto processed: %v", agentRunID, err)
		r.fail(ctx, doc.TenantID, documentID, agentRunID, iterations, "persist_auto_processed_failed")
		return
	}
	_ = r.ToolExecutions.Record(ctx, agentRunID, toolFinalizeDocument,
		map[string]any{"document_id": documentID, "overall_confidence": overallConfidence},
		map[string]any{"status": domain.StatusAutoProcessed}, domain.ToolExecutionSuccess)

	if err := r.AgentRuns.Finish(ctx, agentRunID, domain.AgentRunCompleted, iterations); err != nil {
		log.Printf("agent run %s: finish: %v", agentRunID, err)
	}
	_ = r.Audit.Record(ctx, doc.TenantID, auditActorAgent, "document.auto_processed",
		domain.AuditEntityDocument, documentID,
		map[string]any{"overall_confidence": overallConfidence, "agent_run_id": agentRunID})

	log.Printf("agent run %s: document %s auto-processed (overall confidence %.2f, threshold %.2f)",
		agentRunID, documentID, overallConfidence, threshold)
}

// routeToReview opens a review task, moves the document to
// PENDING_REVIEW, and finishes the agent run as COMPLETED (routing to a
// human is a normal outcome, not a processing failure). Both confidence
// values are optional — see DocumentRepo.MarkPendingReview.
func (r *Runner) routeToReview(ctx context.Context, tenantID, documentID, agentRunID string, iterations int, reason domain.ReviewReason, classificationConfidence, overallConfidence *float64, auditMetadata map[string]any) {
	if err := r.Reviews.Create(ctx, documentID, reason); err != nil {
		log.Printf("agent run %s: create review task: %v", agentRunID, err)
	}
	if err := r.Documents.MarkPendingReview(ctx, documentID, classificationConfidence, overallConfidence); err != nil {
		log.Printf("agent run %s: mark pending review: %v", agentRunID, err)
	}
	if err := r.AgentRuns.Finish(ctx, agentRunID, domain.AgentRunCompleted, iterations); err != nil {
		log.Printf("agent run %s: finish: %v", agentRunID, err)
	}
	auditMetadata["reason"] = reason
	auditMetadata["agent_run_id"] = agentRunID
	_ = r.Audit.Record(ctx, tenantID, auditActorAgent, "document.routed_to_review",
		domain.AuditEntityDocument, documentID, auditMetadata)

	log.Printf("agent run %s: document %s routed to human review (%s)", agentRunID, documentID, reason)
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

// withToolTimeout returns a context bounded by r.ToolTimeout, and a
// cancel func the caller must invoke once the tool call returns (or
// the passed-through parent context and a no-op cancel, if
// ToolTimeout is unset).
func (r *Runner) withToolTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.ToolTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, r.ToolTimeout)
}

// toolFailure classifies an error from an external tool call into the
// ToolExecution status and agent-run failure reason to record — a
// context deadline is a TIMEOUT, not a generic FAILED (NFR-3, SRS
// Feature: Agent Tool Execution — "every tool execution is recorded
// with a status: success, failed, or timeout").
func toolFailure(err error, failedReason, timeoutReason string) (domain.ToolExecutionStatus, string) {
	if errors.Is(err, context.DeadlineExceeded) {
		return domain.ToolExecutionTimeout, timeoutReason
	}
	return domain.ToolExecutionFailed, failedReason
}

// keyFieldsHash returns a deterministic hash of every extracted field
// value, used by check_duplicate to catch near-duplicates an exact
// content-hash match would miss (e.g. the same invoice rescanned,
// different file bytes but identical vendor/amount/date). It returns
// ok=false if any field is missing — a partial extraction isn't a
// reliable basis for matching against another document's full set of
// key fields, and would risk false-positive duplicate flags.
func keyFieldsHash(fields map[string]llm.FieldExtraction) (hash string, ok bool) {
	names := make([]string, 0, len(fields))
	for name, fe := range fields {
		if fe.Value == nil {
			return "", false
		}
		names = append(names, name)
	}
	sort.Strings(names)

	h := sha256.New()
	for _, name := range names {
		fmt.Fprintf(h, "%s=%v\n", name, fields[name].Value)
	}
	return hex.EncodeToString(h.Sum(nil)), true
}
