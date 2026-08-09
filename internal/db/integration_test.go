//go:build integration

package db_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"golang-nextjs/internal/db"
	"golang-nextjs/internal/domain"
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

func newDocument(t *testing.T, repo *db.DocumentRepo, mutate func(*domain.Document)) domain.Document {
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
	require.NoError(t, repo.Create(context.Background(), d))
	return d
}

func TestDocumentRepo_CreateAndGetByID(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := db.NewDocumentRepo(pool)

	created := newDocument(t, repo, nil)

	got, err := repo.GetByID(ctx, devTenantID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, domain.StatusUploaded, got.Status)
	assert.Equal(t, created.ContentHash, got.ContentHash)
	assert.False(t, got.IsDuplicate)
}

func TestDocumentRepo_GetByID_NotFound(t *testing.T) {
	reset(t)
	repo := db.NewDocumentRepo(pool)

	_, err := repo.GetByID(context.Background(), devTenantID, uuid.NewString())
	assert.ErrorIs(t, err, domain.ErrDocumentNotFound)
}

func TestDocumentRepo_GetByID_WrongTenantIsNotFound(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := db.NewDocumentRepo(pool)

	created := newDocument(t, repo, nil)

	// A document only belongs to the tenant it was created under (NFR-5) —
	// GetByID must not leak it to a request for a different tenant.
	_, err := repo.GetByID(ctx, uuid.NewString(), created.ID)
	assert.ErrorIs(t, err, domain.ErrDocumentNotFound)
}

func TestDocumentRepo_GetByIDAny_IgnoresTenant(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := db.NewDocumentRepo(pool)

	created := newDocument(t, repo, nil)

	got, err := repo.GetByIDAny(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
}

func TestDocumentRepo_StatusTransitions(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := db.NewDocumentRepo(pool)

	doc := newDocument(t, repo, nil)

	require.NoError(t, repo.MarkClassified(ctx, doc.ID, domain.DocumentTypeInvoice, 0.95))
	got, err := repo.GetByID(ctx, devTenantID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusClassified, got.Status)
	require.NotNil(t, got.DocumentTypeID)
	assert.Equal(t, domain.DocumentTypeInvoice, *got.DocumentTypeID)
	require.NotNil(t, got.ClassificationConfidence)
	assert.InDelta(t, 0.95, *got.ClassificationConfidence, 0.0001)

	require.NoError(t, repo.MarkExtracted(ctx, doc.ID))
	got, err = repo.GetByID(ctx, devTenantID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusExtracted, got.Status)

	require.NoError(t, repo.MarkValidated(ctx, doc.ID))
	got, err = repo.GetByID(ctx, devTenantID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusValidated, got.Status)

	require.NoError(t, repo.MarkAutoProcessed(ctx, doc.ID, 0.93))
	got, err = repo.GetByID(ctx, devTenantID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusAutoProcessed, got.Status)
	require.NotNil(t, got.OverallConfidence)
	assert.InDelta(t, 0.93, *got.OverallConfidence, 0.0001)

	require.NoError(t, repo.MarkReviewed(ctx, doc.ID))
	got, err = repo.GetByID(ctx, devTenantID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusReviewed, got.Status)
}

func TestDocumentRepo_MarkFailed(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := db.NewDocumentRepo(pool)

	doc := newDocument(t, repo, nil)
	require.NoError(t, repo.MarkFailed(ctx, doc.ID))

	got, err := repo.GetByID(ctx, devTenantID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, got.Status)
}

func TestDocumentRepo_MarkPendingReview_CoalescesNilConfidence(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := db.NewDocumentRepo(pool)

	doc := newDocument(t, repo, nil)
	require.NoError(t, repo.MarkClassified(ctx, doc.ID, domain.DocumentTypeInvoice, 0.8))

	// The low-confidence routing path only passes overallConfidence — a
	// nil classificationConfidence must leave the existing column
	// untouched rather than blanking it out (see MarkPendingReview doc
	// comment).
	overall := 0.5
	require.NoError(t, repo.MarkPendingReview(ctx, doc.ID, nil, &overall))

	got, err := repo.GetByID(ctx, devTenantID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusPendingReview, got.Status)
	require.NotNil(t, got.ClassificationConfidence)
	assert.InDelta(t, 0.8, *got.ClassificationConfidence, 0.0001)
	require.NotNil(t, got.OverallConfidence)
	assert.InDelta(t, 0.5, *got.OverallConfidence, 0.0001)
}

func TestDocumentRepo_MarkDuplicateOf(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := db.NewDocumentRepo(pool)

	original := newDocument(t, repo, nil)
	dup := newDocument(t, repo, func(d *domain.Document) { d.ContentHash = original.ContentHash })

	require.NoError(t, repo.MarkDuplicateOf(ctx, dup.ID, original.ID))

	got, err := repo.GetByID(ctx, devTenantID, dup.ID)
	require.NoError(t, err)
	assert.True(t, got.IsDuplicate)
	require.NotNil(t, got.DuplicateOfDocumentID)
	assert.Equal(t, original.ID, *got.DuplicateOfDocumentID)
}

func TestDocumentRepo_FindByContentHash(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := db.NewDocumentRepo(pool)

	hash := uuid.NewString()
	first := newDocument(t, repo, func(d *domain.Document) { d.ContentHash = hash })
	_ = newDocument(t, repo, func(d *domain.Document) { d.ContentHash = hash })

	got, found, err := repo.FindByContentHash(ctx, devTenantID, hash)
	require.NoError(t, err)
	require.True(t, found)
	// FindByContentHash returns the earliest matching document, so the
	// agent's duplicate check compares a new upload against the
	// original, not another duplicate.
	assert.Equal(t, first.ID, got.ID)

	_, found, err = repo.FindByContentHash(ctx, devTenantID, uuid.NewString())
	require.NoError(t, err)
	assert.False(t, found)
}

func TestExtractedFieldRepo_InsertBatchAndListByDocument(t *testing.T) {
	reset(t)
	ctx := context.Background()
	docs := db.NewDocumentRepo(pool)
	fields := db.NewExtractedFieldRepo(pool)

	doc := newDocument(t, docs, nil)

	require.NoError(t, fields.InsertBatch(ctx, doc.ID, []domain.ExtractedField{
		{DocumentID: doc.ID, FieldName: "vendor_name", Value: "Acme Corp", Confidence: 0.9, Source: domain.ExtractedFieldSourceExtraction},
		{DocumentID: doc.ID, FieldName: "total_amount", Value: nil, Confidence: 0, Source: domain.ExtractedFieldSourceExtraction},
	}))

	got, err := fields.ListByDocument(ctx, doc.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)

	byName := make(map[string]domain.ExtractedField, len(got))
	for _, f := range got {
		byName[f.FieldName] = f
	}
	assert.Equal(t, "Acme Corp", byName["vendor_name"].Value)
	assert.Nil(t, byName["total_amount"].Value)

	// A reviewer correction shadows the original extraction: ListByDocument
	// returns the most recent row per field, both rows remain in the table.
	require.NoError(t, fields.InsertBatch(ctx, doc.ID, []domain.ExtractedField{
		{DocumentID: doc.ID, FieldName: "vendor_name", Value: "Corrected Corp", Confidence: 1.0, Source: domain.ExtractedFieldSourceReviewCorrection},
	}))

	got, err = fields.ListByDocument(ctx, doc.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, f := range got {
		if f.FieldName == "vendor_name" {
			assert.Equal(t, "Corrected Corp", f.Value)
			assert.Equal(t, domain.ExtractedFieldSourceReviewCorrection, f.Source)
		}
	}
}

func TestReviewTaskRepo_CreateGetResolveListPending(t *testing.T) {
	reset(t)
	ctx := context.Background()
	docs := db.NewDocumentRepo(pool)
	reviews := db.NewReviewTaskRepo(pool)

	doc := newDocument(t, docs, nil)
	require.NoError(t, reviews.Create(ctx, doc.ID, domain.ReviewReasonLowConfidence))

	task, err := reviews.GetPendingByDocument(ctx, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReviewReasonLowConfidence, task.Reason)
	assert.Equal(t, domain.ReviewStatusPending, task.Status)

	items, err := reviews.ListPendingByTenant(ctx, devTenantID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, task.ID, items[0].ReviewTaskID)

	require.NoError(t, reviews.Resolve(ctx, task.ID, domain.ReviewStatusApproved, devUserID, "looks good"))

	items, err = reviews.ListPendingByTenant(ctx, devTenantID)
	require.NoError(t, err)
	assert.Empty(t, items, "resolved tasks must drop out of the pending queue")

	_, err = reviews.GetPendingByDocument(ctx, doc.ID)
	assert.ErrorIs(t, err, db.ErrReviewTaskNotFound)
}

func TestUserRepo_GetRole(t *testing.T) {
	reset(t)
	repo := db.NewUserRepo(pool)

	role, err := repo.GetRole(context.Background(), devUserID)
	require.NoError(t, err)
	assert.Equal(t, "admin", role)
}

func TestDocumentTypeRepo_GetFieldSchemaAndThreshold(t *testing.T) {
	reset(t)
	ctx := context.Background()
	repo := db.NewDocumentTypeRepo(pool)

	schema, err := repo.GetFieldSchema(ctx, domain.DocumentTypeInvoice)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"vendor_name":  "string",
		"total_amount": "number",
		"invoice_date": "date",
	}, schema)

	threshold, err := repo.GetAutoProcessThreshold(ctx, domain.DocumentTypeInvoice)
	require.NoError(t, err)
	assert.InDelta(t, 0.9, threshold, 0.0001)
}

func TestAgentRunRepo_FinishAndListByDocument(t *testing.T) {
	reset(t)
	ctx := context.Background()
	docs := db.NewDocumentRepo(pool)
	runs := db.NewAgentRunRepo(pool)

	doc := newDocument(t, docs, nil)
	_, agentRunID, ok, err := db.ClaimNextUploaded(ctx, pool, 10)
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, runs.Finish(ctx, agentRunID, domain.AgentRunCompleted, 7))

	list, err := runs.ListByDocument(ctx, doc.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, domain.AgentRunCompleted, list[0].Status)
	assert.Equal(t, 7, list[0].IterationCount)
	assert.Equal(t, 10, list[0].MaxIterations)
	assert.NotNil(t, list[0].FinishedAt)
}

func TestAuditLogRepo_Record(t *testing.T) {
	reset(t)
	repo := db.NewAuditLogRepo(pool)

	err := repo.Record(context.Background(), devTenantID, "agent", "document.classified",
		domain.AuditEntityDocument, uuid.NewString(), map[string]any{"confidence": 0.95})
	require.NoError(t, err)
}

func TestToolExecutionRepo_RecordAndListByAgentRun(t *testing.T) {
	reset(t)
	ctx := context.Background()
	docs := db.NewDocumentRepo(pool)
	execs := db.NewToolExecutionRepo(pool)

	newDocument(t, docs, nil)
	_, agentRunID, ok, err := db.ClaimNextUploaded(ctx, pool, 10)
	require.NoError(t, err)
	require.True(t, ok)

	require.NoError(t, execs.Record(ctx, agentRunID, "run_ocr",
		map[string]any{"document_id": agentRunID}, map[string]any{"text_length": 42}, domain.ToolExecutionSuccess))

	list, err := execs.ListByAgentRun(ctx, agentRunID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "run_ocr", list[0].ToolName)
	assert.Equal(t, domain.ToolExecutionSuccess, list[0].Status)
	assert.EqualValues(t, 42, list[0].Output["text_length"])
}

func TestClaimNextUploaded(t *testing.T) {
	reset(t)
	ctx := context.Background()
	docs := db.NewDocumentRepo(pool)

	// Nothing to claim yet.
	_, _, ok, err := db.ClaimNextUploaded(ctx, pool, 10)
	require.NoError(t, err)
	assert.False(t, ok)

	older := newDocument(t, docs, nil)
	_ = newDocument(t, docs, nil)

	documentID, agentRunID, ok, err := db.ClaimNextUploaded(ctx, pool, 6)
	require.NoError(t, err)
	require.True(t, ok)
	// The oldest UPLOADED document (by created_at) is claimed first.
	assert.Equal(t, older.ID, documentID)
	assert.NotEmpty(t, agentRunID)

	runs := db.NewAgentRunRepo(pool)
	list, err := runs.ListByDocument(ctx, documentID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, domain.AgentRunRunning, list[0].Status)
	assert.Equal(t, 6, list[0].MaxIterations)

	// A document with an already-open RUNNING agent run must not be
	// claimed again — the second call picks the other uploaded document.
	_, secondAgentRunID, ok, err := db.ClaimNextUploaded(ctx, pool, 6)
	require.NoError(t, err)
	require.True(t, ok)
	assert.NotEqual(t, agentRunID, secondAgentRunID)

	// Both UPLOADED documents now have RUNNING agent runs, so nothing
	// remains to claim.
	_, _, ok, err = db.ClaimNextUploaded(ctx, pool, 6)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestMigrate_IsIdempotent(t *testing.T) {
	// Applying migrations again against an already-migrated database
	// must be a no-op, not an error (workers and the API both call
	// db.Migrate on every startup).
	require.NoError(t, db.Migrate(context.Background(), pool))
}
