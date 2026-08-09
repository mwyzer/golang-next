//go:build integration

package agent_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"golang-nextjs/internal/agent"
	"golang-nextjs/internal/db"
	"golang-nextjs/internal/domain"
	"golang-nextjs/internal/providers/llm"
	"golang-nextjs/internal/providers/ocr"
	"golang-nextjs/internal/testutil"
)

// Fixed IDs seeded by db/migrations/000002_seed_dev_data.up.sql.
const (
	devTenantID = "00000000-0000-0000-0000-000000000001"
	devUserID   = "00000000-0000-0000-0000-000000000002"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()
	p, err := testutil.StartShared(ctx)
	if err != nil {
		panic(err)
	}
	pool = p

	code := m.Run()
	testutil.StopShared(ctx)
	os.Exit(code)
}

func reset(t *testing.T) {
	t.Helper()
	require.NoError(t, testutil.Reset(context.Background(), pool))
}

// fakeLLM lets each test dictate exactly what classification/extraction
// the agent sees, instead of depending on llm.StubProvider's keyword
// and "Label: value" text heuristics.
type fakeLLM struct {
	classifyResult llm.ClassifyResult
	classifyErr    error
	extractFields  map[string]llm.FieldExtraction
	extractErr     error
}

func (f fakeLLM) Classify(context.Context, string) (llm.ClassifyResult, error) {
	return f.classifyResult, f.classifyErr
}

func (f fakeLLM) Extract(context.Context, string, map[string]string) (llm.ExtractResult, error) {
	return llm.ExtractResult{Fields: f.extractFields}, f.extractErr
}

type fakeOCR struct {
	text string
	err  error
}

func (f fakeOCR) ExtractText(context.Context, string) (string, error) {
	return f.text, f.err
}

// validInvoiceFields satisfies the seeded "invoice" schema
// ({"vendor_name":"string","total_amount":"number","invoice_date":"date"})
// well enough to pass validation.Validate.
func validInvoiceFields(confidence float64) map[string]llm.FieldExtraction {
	return map[string]llm.FieldExtraction{
		"vendor_name":  {Value: "Acme Corp", Confidence: confidence},
		"total_amount": {Value: 100.0, Confidence: confidence},
		"invoice_date": {Value: "2024-01-15", Confidence: confidence},
	}
}

func newRunner(ocrProvider ocr.Provider, llmProvider llm.Provider, maxIterations int) *agent.Runner {
	return &agent.Runner{
		Pool:            pool,
		Documents:       db.NewDocumentRepo(pool),
		DocumentTypes:   db.NewDocumentTypeRepo(pool),
		ExtractedFields: db.NewExtractedFieldRepo(pool),
		AgentRuns:       db.NewAgentRunRepo(pool),
		ToolExecutions:  db.NewToolExecutionRepo(pool),
		Reviews:         db.NewReviewTaskRepo(pool),
		Audit:           db.NewAuditLogRepo(pool),
		OCR:             ocrProvider,
		LLM:             llmProvider,
		MaxIterations:   maxIterations,
	}
}

func uploadDocument(t *testing.T, mutate func(*domain.Document)) domain.Document {
	t.Helper()
	d := domain.Document{
		ID:            uuid.NewString(),
		TenantID:      devTenantID,
		UploadedBy:    devUserID,
		Status:        domain.StatusUploaded,
		FilePath:      "/data/uploads/" + uuid.NewString(),
		MimeType:      "application/pdf",
		FileSizeBytes: 1024,
		ContentHash:   uuid.NewString(),
	}
	if mutate != nil {
		mutate(&d)
	}
	require.NoError(t, db.NewDocumentRepo(pool).Create(context.Background(), d))
	return d
}

// auditReason returns the most recent audit_logs.metadata->>'reason' for
// an entity. AuditLogRepo only exposes Record (audit logs are
// write-only by design), so this queries the table directly.
func auditReason(t *testing.T, entityType domain.AuditEntityType, entityID string) string {
	t.Helper()
	var reason *string
	err := pool.QueryRow(context.Background(), `
		SELECT metadata->>'reason' FROM audit_logs
		WHERE entity_type = $1 AND entity_id = $2
		ORDER BY created_at DESC LIMIT 1`, entityType, entityID,
	).Scan(&reason)
	require.NoError(t, err)
	require.NotNil(t, reason)
	return *reason
}

func TestRunner_PollOnce_NothingToClaim(t *testing.T) {
	reset(t)
	r := newRunner(fakeOCR{}, fakeLLM{}, 10)

	processed, err := r.PollOnce(context.Background())
	require.NoError(t, err)
	assert.False(t, processed)
}

func TestRunner_AutoProcess_HappyPath(t *testing.T) {
	reset(t)
	ctx := context.Background()
	doc := uploadDocument(t, nil)

	r := newRunner(
		fakeOCR{text: "invoice text"},
		fakeLLM{
			classifyResult: llm.ClassifyResult{DocumentType: domain.DocumentTypeInvoice, Confidence: 0.95},
			extractFields:  validInvoiceFields(0.95),
		},
		10,
	)

	processed, err := r.PollOnce(ctx)
	require.NoError(t, err)
	assert.True(t, processed)

	got, err := db.NewDocumentRepo(pool).GetByID(ctx, devTenantID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusAutoProcessed, got.Status)
	require.NotNil(t, got.OverallConfidence)
	assert.InDelta(t, 0.95, *got.OverallConfidence, 0.0001)
	assert.False(t, got.IsDuplicate)

	fields, err := db.NewExtractedFieldRepo(pool).ListByDocument(ctx, doc.ID)
	require.NoError(t, err)
	assert.Len(t, fields, 3)

	runs, err := db.NewAgentRunRepo(pool).ListByDocument(ctx, doc.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, domain.AgentRunCompleted, runs[0].Status)
	assert.Equal(t, 7, runs[0].IterationCount, "the full 7-step pipeline should have run")
}

func TestRunner_RouteToReview_UnknownType(t *testing.T) {
	reset(t)
	ctx := context.Background()
	doc := uploadDocument(t, nil)

	r := newRunner(
		fakeOCR{text: "some illegible scrawl"},
		fakeLLM{classifyResult: llm.ClassifyResult{DocumentType: domain.DocumentTypeUnknown, Confidence: 0.2}},
		10,
	)

	processed, err := r.PollOnce(ctx)
	require.NoError(t, err)
	assert.True(t, processed)

	got, err := db.NewDocumentRepo(pool).GetByID(ctx, devTenantID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPendingReview, got.Status)
	require.NotNil(t, got.ClassificationConfidence)
	assert.InDelta(t, 0.2, *got.ClassificationConfidence, 0.0001)
	assert.Nil(t, got.OverallConfidence)

	task, err := db.NewReviewTaskRepo(pool).GetPendingByDocument(ctx, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReviewReasonUnknownType, task.Reason)

	runs, err := db.NewAgentRunRepo(pool).ListByDocument(ctx, doc.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	// Routing to review is a successful outcome, not a failure: the run
	// finishes COMPLETED, having only executed run_ocr + classify_document.
	assert.Equal(t, domain.AgentRunCompleted, runs[0].Status)
	assert.Equal(t, 2, runs[0].IterationCount)
}

func TestRunner_RouteToReview_ValidationFailed(t *testing.T) {
	reset(t)
	ctx := context.Background()
	doc := uploadDocument(t, nil)

	fields := validInvoiceFields(0.95)
	fields["total_amount"] = llm.FieldExtraction{Value: nil, Confidence: 0} // required field missing

	r := newRunner(
		fakeOCR{text: "invoice text"},
		fakeLLM{
			classifyResult: llm.ClassifyResult{DocumentType: domain.DocumentTypeInvoice, Confidence: 0.95},
			extractFields:  fields,
		},
		10,
	)

	processed, err := r.PollOnce(ctx)
	require.NoError(t, err)
	assert.True(t, processed)

	got, err := db.NewDocumentRepo(pool).GetByID(ctx, devTenantID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPendingReview, got.Status)
	// classify_document already persisted a classification_confidence
	// before validation ran; routeToReview passes nil for it, which
	// COALESCEs to the existing value rather than blanking it.
	require.NotNil(t, got.ClassificationConfidence)
	assert.InDelta(t, 0.95, *got.ClassificationConfidence, 0.0001)
	assert.Nil(t, got.OverallConfidence)

	task, err := db.NewReviewTaskRepo(pool).GetPendingByDocument(ctx, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReviewReasonValidationFailed, task.Reason)

	runs, err := db.NewAgentRunRepo(pool).ListByDocument(ctx, doc.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, domain.AgentRunCompleted, runs[0].Status)
	assert.Equal(t, 4, runs[0].IterationCount)
}

func TestRunner_RouteToReview_Duplicate(t *testing.T) {
	reset(t)
	ctx := context.Background()

	sharedHash := uuid.NewString()
	// The earlier document must not be UPLOADED, or ClaimNextUploaded
	// could pick it up instead of the actual duplicate under test.
	earlier := uploadDocument(t, func(d *domain.Document) { d.ContentHash = sharedHash })
	require.NoError(t, db.NewDocumentRepo(pool).MarkAutoProcessed(ctx, earlier.ID, 0.95))

	dup := uploadDocument(t, func(d *domain.Document) { d.ContentHash = sharedHash })

	r := newRunner(
		fakeOCR{text: "invoice text"},
		fakeLLM{
			classifyResult: llm.ClassifyResult{DocumentType: domain.DocumentTypeInvoice, Confidence: 0.95},
			extractFields:  validInvoiceFields(0.95),
		},
		10,
	)

	processed, err := r.PollOnce(ctx)
	require.NoError(t, err)
	assert.True(t, processed)

	got, err := db.NewDocumentRepo(pool).GetByID(ctx, devTenantID, dup.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPendingReview, got.Status)
	assert.True(t, got.IsDuplicate)
	require.NotNil(t, got.DuplicateOfDocumentID)
	assert.Equal(t, earlier.ID, *got.DuplicateOfDocumentID)

	task, err := db.NewReviewTaskRepo(pool).GetPendingByDocument(ctx, dup.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReviewReasonDuplicate, task.Reason)

	runs, err := db.NewAgentRunRepo(pool).ListByDocument(ctx, dup.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, domain.AgentRunCompleted, runs[0].Status)
	assert.Equal(t, 5, runs[0].IterationCount)
}

func TestRunner_RouteToReview_LowConfidence(t *testing.T) {
	reset(t)
	ctx := context.Background()
	doc := uploadDocument(t, nil)

	// Average of classify (0.5) + 3 field confidences (0.5 each) = 0.5,
	// well under the invoice type's 0.9 auto-process threshold.
	r := newRunner(
		fakeOCR{text: "invoice text"},
		fakeLLM{
			classifyResult: llm.ClassifyResult{DocumentType: domain.DocumentTypeInvoice, Confidence: 0.5},
			extractFields:  validInvoiceFields(0.5),
		},
		10,
	)

	processed, err := r.PollOnce(ctx)
	require.NoError(t, err)
	assert.True(t, processed)

	got, err := db.NewDocumentRepo(pool).GetByID(ctx, devTenantID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPendingReview, got.Status)
	require.NotNil(t, got.OverallConfidence)
	assert.InDelta(t, 0.5, *got.OverallConfidence, 0.0001)

	task, err := db.NewReviewTaskRepo(pool).GetPendingByDocument(ctx, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReviewReasonLowConfidence, task.Reason)

	runs, err := db.NewAgentRunRepo(pool).ListByDocument(ctx, doc.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, domain.AgentRunCompleted, runs[0].Status)
	assert.Equal(t, 6, runs[0].IterationCount)
}

func TestRunner_MaxIterationsExceeded(t *testing.T) {
	reset(t)
	ctx := context.Background()
	doc := uploadDocument(t, nil)

	// MaxIterations=2 lets run_ocr (1) and classify_document (2)
	// complete; extract_fields's pre-check increments to 3, which trips
	// the cap before any LLM.Extract call happens.
	r := newRunner(
		fakeOCR{text: "invoice text"},
		fakeLLM{classifyResult: llm.ClassifyResult{DocumentType: domain.DocumentTypeInvoice, Confidence: 0.95}},
		2,
	)

	processed, err := r.PollOnce(ctx)
	require.NoError(t, err)
	assert.True(t, processed)

	got, err := db.NewDocumentRepo(pool).GetByID(ctx, devTenantID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, got.Status)

	runs, err := db.NewAgentRunRepo(pool).ListByDocument(ctx, doc.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, domain.AgentRunFailed, runs[0].Status)
	assert.Equal(t, 3, runs[0].IterationCount)
	assert.Equal(t, 2, runs[0].MaxIterations)

	assert.Equal(t, "max_iterations_exceeded", auditReason(t, domain.AuditEntityAgentRun, runs[0].ID))

	// No review task should have been opened — this is a failure, not a
	// routing decision.
	_, err = db.NewReviewTaskRepo(pool).GetPendingByDocument(ctx, doc.ID)
	assert.ErrorIs(t, err, db.ErrReviewTaskNotFound)
}

func TestRunner_OCRFailure(t *testing.T) {
	reset(t)
	ctx := context.Background()
	doc := uploadDocument(t, nil)

	r := newRunner(fakeOCR{err: assert.AnError}, fakeLLM{}, 10)

	processed, err := r.PollOnce(ctx)
	require.NoError(t, err)
	assert.True(t, processed)

	got, err := db.NewDocumentRepo(pool).GetByID(ctx, devTenantID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, got.Status)

	runs, err := db.NewAgentRunRepo(pool).ListByDocument(ctx, doc.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, domain.AgentRunFailed, runs[0].Status)
	assert.Equal(t, "ocr_failed", auditReason(t, domain.AuditEntityAgentRun, runs[0].ID))

	execs, err := db.NewToolExecutionRepo(pool).ListByAgentRun(ctx, runs[0].ID)
	require.NoError(t, err)
	require.Len(t, execs, 1)
	assert.Equal(t, "run_ocr", execs[0].ToolName)
	assert.Equal(t, domain.ToolExecutionFailed, execs[0].Status)
}
