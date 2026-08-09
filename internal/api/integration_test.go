//go:build integration

package api_test

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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"golang-nextjs/internal/api"
	"golang-nextjs/internal/db"
	"golang-nextjs/internal/domain"
	"golang-nextjs/internal/storage"
	"golang-nextjs/internal/testutil"
)

// Fixed IDs seeded by db/migrations/000002_seed_dev_data.up.sql. The
// seeded dev user has role "admin", which satisfies the reviewer-only
// endpoints' RequireRole check.
const (
	devTenantID = "00000000-0000-0000-0000-000000000001"
	devUserID   = "00000000-0000-0000-0000-000000000002"
	apiToken    = "test-token"
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

func newTestServer(t *testing.T) *httptest.Server {
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
		MaxUploadSize:   20 * 1024 * 1024,
		APIToken:        apiToken,
	}

	srv := httptest.NewServer(api.NewRouter(deps))
	t.Cleanup(srv.Close)
	return srv
}

// uploadRequest builds a multipart POST /api/v1/documents request with
// an explicit Content-Type on the file part, so tests can exercise
// ValidateUpload's MIME allowlist without relying on content sniffing.
func uploadRequest(t *testing.T, srv *httptest.Server, token, filename, contentType string, content []byte) *http.Response {
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
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func authedRequest(t *testing.T, method, url string, body []byte) *http.Response {
	t.Helper()
	var r *bytes.Reader
	if body != nil {
		r = bytes.NewReader(body)
	} else {
		r = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, r)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+apiToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(v))
}

// createDocumentAt inserts a document directly via the repo (bypassing
// the upload endpoint) so review-flow tests can start from an arbitrary
// state without re-driving the whole agent pipeline through HTTP.
func createDocumentAt(t *testing.T, status domain.DocumentStatus) domain.Document {
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
	repo := db.NewDocumentRepo(pool)
	require.NoError(t, repo.Create(context.Background(), d))
	if status != domain.StatusUploaded {
		require.NoError(t, repo.MarkPendingReview(context.Background(), d.ID, nil, nil))
	}
	return d
}

func TestHealthz(t *testing.T) {
	srv := newTestServer(t)

	resp, err := http.Get(srv.URL + "/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	var body map[string]string
	decodeJSON(t, resp, &body)
	assert.Equal(t, "ok", body["status"])
}

func TestRequireAuth(t *testing.T) {
	reset(t)
	srv := newTestServer(t)

	t.Run("missing token", func(t *testing.T) {
		resp, err := http.Get(srv.URL + "/api/v1/documents/" + uuid.NewString())
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("wrong token", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/documents/"+uuid.NewString(), nil)
		req.Header.Set("Authorization", "Bearer wrong-token")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

func TestUploadAndGetDocument(t *testing.T) {
	reset(t)
	srv := newTestServer(t)

	resp := uploadRequest(t, srv, apiToken, "invoice.pdf", "application/pdf", []byte("%PDF-1.4 fake invoice content"))
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	var uploaded struct {
		DocumentID string `json:"document_id"`
		Status     string `json:"status"`
	}
	decodeJSON(t, resp, &uploaded)
	assert.NotEmpty(t, uploaded.DocumentID)
	assert.Equal(t, string(domain.StatusUploaded), uploaded.Status)

	getResp := authedRequest(t, http.MethodGet, srv.URL+"/api/v1/documents/"+uploaded.DocumentID, nil)
	require.Equal(t, http.StatusOK, getResp.StatusCode)

	var got struct {
		DocumentID string         `json:"document_id"`
		Status     string         `json:"status"`
		Fields     map[string]any `json:"fields"`
	}
	decodeJSON(t, getResp, &got)
	assert.Equal(t, uploaded.DocumentID, got.DocumentID)
	assert.Equal(t, string(domain.StatusUploaded), got.Status)
	assert.Empty(t, got.Fields)
}

func TestUpload_UnsupportedFileType(t *testing.T) {
	reset(t)
	srv := newTestServer(t)

	resp := uploadRequest(t, srv, apiToken, "notes.txt", "text/plain", []byte("just some text"))
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var body map[string]map[string]string
	decodeJSON(t, resp, &body)
	assert.Equal(t, "UNSUPPORTED_FILE_TYPE", body["error"]["code"])
}

func TestUpload_MissingFile(t *testing.T) {
	reset(t)
	srv := newTestServer(t)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	require.NoError(t, w.Close()) // no "file" field written

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/documents", &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiToken)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	var body map[string]map[string]string
	decodeJSON(t, resp, &body)
	assert.Equal(t, "MISSING_FILE", body["error"]["code"])
}

func TestGetDocument_NotFound(t *testing.T) {
	reset(t)
	srv := newTestServer(t)

	resp := authedRequest(t, http.MethodGet, srv.URL+"/api/v1/documents/"+uuid.NewString(), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestReviewFlow_Approve(t *testing.T) {
	reset(t)
	srv := newTestServer(t)

	doc := createDocumentAt(t, domain.StatusPendingReview)
	require.NoError(t, db.NewReviewTaskRepo(pool).Create(context.Background(), doc.ID, domain.ReviewReasonLowConfidence))

	// The document should show up in the review queue before it's resolved.
	queueResp := authedRequest(t, http.MethodGet, srv.URL+"/api/v1/review-queue", nil)
	require.Equal(t, http.StatusOK, queueResp.StatusCode)
	var queue struct {
		ReviewQueue []map[string]any `json:"review_queue"`
	}
	decodeJSON(t, queueResp, &queue)
	require.Len(t, queue.ReviewQueue, 1)
	assert.Equal(t, doc.ID, queue.ReviewQueue[0]["document_id"])

	body, _ := json.Marshal(map[string]any{"decision": "approve", "notes": "looks fine"})
	resp := authedRequest(t, http.MethodPost, srv.URL+"/api/v1/documents/"+doc.ID+"/review", body)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		DocumentID   string `json:"document_id"`
		ReviewStatus string `json:"review_status"`
	}
	decodeJSON(t, resp, &result)
	assert.Equal(t, "APPROVED", result.ReviewStatus)

	got, err := db.NewDocumentRepo(pool).GetByID(context.Background(), devTenantID, doc.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusReviewed, got.Status)

	// Resolved tasks drop out of the queue.
	queueResp2 := authedRequest(t, http.MethodGet, srv.URL+"/api/v1/review-queue", nil)
	decodeJSON(t, queueResp2, &queue)
	assert.Empty(t, queue.ReviewQueue)
}

func TestReviewFlow_CorrectRequiresFields(t *testing.T) {
	reset(t)
	srv := newTestServer(t)

	doc := createDocumentAt(t, domain.StatusPendingReview)
	require.NoError(t, db.NewReviewTaskRepo(pool).Create(context.Background(), doc.ID, domain.ReviewReasonValidationFailed))

	body, _ := json.Marshal(map[string]any{"decision": "correct", "corrected_fields": map[string]any{}})
	resp := authedRequest(t, http.MethodPost, srv.URL+"/api/v1/documents/"+doc.ID+"/review", body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var respBody map[string]map[string]string
	decodeJSON(t, resp, &respBody)
	assert.Equal(t, "MISSING_CORRECTIONS", respBody["error"]["code"])
}

func TestReviewFlow_CorrectPersistsFields(t *testing.T) {
	reset(t)
	srv := newTestServer(t)

	doc := createDocumentAt(t, domain.StatusPendingReview)
	require.NoError(t, db.NewReviewTaskRepo(pool).Create(context.Background(), doc.ID, domain.ReviewReasonValidationFailed))

	body, _ := json.Marshal(map[string]any{
		"decision":         "correct",
		"corrected_fields": map[string]any{"vendor_name": "Corrected Vendor"},
	})
	resp := authedRequest(t, http.MethodPost, srv.URL+"/api/v1/documents/"+doc.ID+"/review", body)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	fields, err := db.NewExtractedFieldRepo(pool).ListByDocument(context.Background(), doc.ID)
	require.NoError(t, err)
	require.Len(t, fields, 1)
	assert.Equal(t, "Corrected Vendor", fields[0].Value)
	assert.Equal(t, domain.ExtractedFieldSourceReviewCorrection, fields[0].Source)
}

func TestReviewFlow_NotReviewableWhenNotPending(t *testing.T) {
	reset(t)
	srv := newTestServer(t)

	// Still UPLOADED — never routed to review, so there's no open task.
	doc := createDocumentAt(t, domain.StatusUploaded)

	body, _ := json.Marshal(map[string]any{"decision": "approve"})
	resp := authedRequest(t, http.MethodPost, srv.URL+"/api/v1/documents/"+doc.ID+"/review", body)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)

	var respBody map[string]map[string]string
	decodeJSON(t, resp, &respBody)
	assert.Equal(t, "NOT_REVIEWABLE", respBody["error"]["code"])
}

func TestRequireRole_ForbidsNonReviewers(t *testing.T) {
	reset(t)
	srv := newTestServer(t)

	// The dev-seeded user is "admin" by default (satisfies RequireRole);
	// temporarily downgrade it to exercise the 403 path, then restore it
	// so later tests relying on admin access aren't affected.
	ctx := context.Background()
	_, err := pool.Exec(ctx, `UPDATE users SET role = 'uploader' WHERE id = $1`, devUserID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `UPDATE users SET role = 'admin' WHERE id = $1`, devUserID)
	})

	resp := authedRequest(t, http.MethodGet, srv.URL+"/api/v1/review-queue", nil)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}
