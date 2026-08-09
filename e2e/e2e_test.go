//go:build e2e

// Package e2e_test drives the real HTTP API together with a real
// worker polling loop (agent.Runner using the actual stub OCR/LLM
// providers, not test fakes) against a testcontainers Postgres — the
// same wiring cmd/api and cmd/worker use, minus separate OS processes.
// It exercises docs/TESTING.md's "End-to-End: Upload -> process ->
// review -> completed, through the API" test level.
package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"golang-nextjs/internal/agent"
	"golang-nextjs/internal/api"
	"golang-nextjs/internal/auth"
	"golang-nextjs/internal/db"
	"golang-nextjs/internal/providers/llm"
	"golang-nextjs/internal/providers/ocr"
	"golang-nextjs/internal/storage"
	"golang-nextjs/internal/testutil"
)

const apiToken = "e2e-token"

// devUserID matches the seeded row in
// db/migrations/000002_seed_dev_data.up.sql.
const devUserID = "00000000-0000-0000-0000-000000000002"

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	ctx := context.Background()
	p, err := testutil.StartShared(ctx)
	if err != nil {
		panic(err)
	}
	pool = p

	// Auth is per-user (internal/api/middleware.go), so bootstrap the
	// seeded dev user's token the same way cmd/api/main.go does from
	// API_TOKEN.
	if err := db.NewUserRepo(pool).SetTokenHash(ctx, devUserID, auth.HashToken(apiToken)); err != nil {
		panic(err)
	}

	code := m.Run()
	testutil.StopShared(ctx)
	os.Exit(code)
}

func reset(t *testing.T) {
	t.Helper()
	require.NoError(t, testutil.Reset(context.Background(), pool))
}

// startStack wires up a real HTTP server (api.NewRouter, matching
// cmd/api/main.go) and a real worker loop (agent.Runner.PollOnce in a
// goroutine, matching cmd/worker/main.go's poll-and-backoff loop)
// against the shared pool, using the production stub OCR/LLM providers
// rather than test fakes.
func startStack(t *testing.T) *httptest.Server {
	t.Helper()

	store, err := storage.NewLocalStore(t.TempDir())
	require.NoError(t, err)

	deps := api.Deps{
		Repo:            db.NewDocumentRepo(pool),
		AgentRuns:       db.NewAgentRunRepo(pool),
		ToolExecs:       db.NewToolExecutionRepo(pool),
		ExtractedFields: db.NewExtractedFieldRepo(pool),
		Reviews:         db.NewReviewTaskRepo(pool),
		Users:           db.NewUserRepo(pool),
		Audit:           db.NewAuditLogRepo(pool),
		Store:           store,
		MaxUploadSize:   20 << 20,
	}
	srv := httptest.NewServer(api.NewRouter(deps))
	t.Cleanup(srv.Close)

	runner := &agent.Runner{
		Pool:            pool,
		Documents:       db.NewDocumentRepo(pool),
		DocumentTypes:   db.NewDocumentTypeRepo(pool),
		ExtractedFields: db.NewExtractedFieldRepo(pool),
		AgentRuns:       db.NewAgentRunRepo(pool),
		ToolExecutions:  db.NewToolExecutionRepo(pool),
		Reviews:         db.NewReviewTaskRepo(pool),
		Audit:           db.NewAuditLogRepo(pool),
		OCR:             ocr.StubProvider{Store: store},
		LLM:             llm.StubProvider{},
		MaxIterations:   10,
	}

	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if processed, _ := runner.PollOnce(ctx); !processed {
				time.Sleep(25 * time.Millisecond)
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		<-stopped
	})

	return srv
}

func uploadFile(t *testing.T, srv *httptest.Server, filename, contentType string, content []byte) string {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="file"; filename=%q`, filename)},
		"Content-Type":        {contentType},
	})
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/documents", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiToken)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var body struct {
		DocumentID string `json:"document_id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return body.DocumentID
}

func getDocument(t *testing.T, srv *httptest.Server, id string) map[string]any {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/documents/"+id, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+apiToken)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return body
}

// waitForStatus polls GET /documents/{id} until its status matches one
// of want, standing in for a client (or the web UI) refreshing while
// the worker processes the document asynchronously.
func waitForStatus(t *testing.T, srv *httptest.Server, id string, want ...string) map[string]any {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	var last map[string]any
	for time.Now().Before(deadline) {
		last = getDocument(t, srv, id)
		status, _ := last["status"].(string)
		for _, w := range want {
			if status == w {
				return last
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for document %s to reach status in %v, last seen: %v", id, want, last)
	return nil
}

func TestE2E_UploadAutoProcessesThroughTheAPI(t *testing.T) {
	reset(t)
	srv := startStack(t)

	content := []byte("This is an invoice.\n" +
		"Vendor Name: Acme Corp\n" +
		"Total Amount: 250.00\n" +
		"Invoice Date: 2024-01-15\n")

	docID := uploadFile(t, srv, "invoice.pdf", "application/pdf", content)

	doc := waitForStatus(t, srv, docID, "AUTO_PROCESSED", "PENDING_REVIEW", "FAILED")
	require.Equal(t, "AUTO_PROCESSED", doc["status"],
		"expected the stub classify/extract/validate pipeline to clear every gate")

	fields, ok := doc["fields"].(map[string]any)
	require.True(t, ok)
	vendor, ok := fields["vendor_name"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Acme Corp", vendor["value"])
}

func TestE2E_UploadReviewCompletesThroughTheAPI(t *testing.T) {
	reset(t)
	srv := startStack(t)

	// Missing "Invoice Date" trips validate_extraction's field_required
	// check, routing the document to human review instead of
	// auto-processing.
	content := []byte("This is an invoice.\n" +
		"Vendor Name: Acme Corp\n" +
		"Total Amount: 250.00\n")

	docID := uploadFile(t, srv, "invoice.pdf", "application/pdf", content)

	doc := waitForStatus(t, srv, docID, "PENDING_REVIEW", "AUTO_PROCESSED", "FAILED")
	require.Equal(t, "PENDING_REVIEW", doc["status"],
		"a missing required field should route to review, not auto-process")

	// The document should be visible in the review queue before it's resolved.
	queueReq, err := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/review-queue", nil)
	require.NoError(t, err)
	queueReq.Header.Set("Authorization", "Bearer "+apiToken)
	queueResp, err := http.DefaultClient.Do(queueReq)
	require.NoError(t, err)
	var queue struct {
		ReviewQueue []map[string]any `json:"review_queue"`
	}
	require.NoError(t, json.NewDecoder(queueResp.Body).Decode(&queue))
	queueResp.Body.Close()

	found := false
	for _, item := range queue.ReviewQueue {
		if item["document_id"] == docID {
			found = true
			require.Equal(t, "VALIDATION_FAILED", item["reason"])
		}
	}
	require.True(t, found, "uploaded document should appear in the review queue")

	// A reviewer approves it through the same API a real client would call.
	body, err := json.Marshal(map[string]any{"decision": "approve", "notes": "e2e approval"})
	require.NoError(t, err)
	reviewReq, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/documents/"+docID+"/review", bytes.NewReader(body))
	require.NoError(t, err)
	reviewReq.Header.Set("Authorization", "Bearer "+apiToken)
	reviewReq.Header.Set("Content-Type", "application/json")
	reviewResp, err := http.DefaultClient.Do(reviewReq)
	require.NoError(t, err)
	defer reviewResp.Body.Close()
	require.Equal(t, http.StatusOK, reviewResp.StatusCode)

	final := waitForStatus(t, srv, docID, "REVIEWED")
	require.Equal(t, "REVIEWED", final["status"])
}
