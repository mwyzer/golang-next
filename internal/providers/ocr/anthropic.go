package ocr

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"golang-nextjs/internal/storage"
)

// AnthropicProvider extracts text via Claude's vision input rather than
// a dedicated OCR vendor — it reuses the same ANTHROPIC_API_KEY as
// llm.AnthropicProvider (internal/providers/llm/anthropic.go), so
// choosing Anthropic for both resolves the LLM and OCR PRD open
// questions with one vendor and one key.
type AnthropicProvider struct {
	client anthropic.Client
	model  string
	store  storage.Store
}

// NewAnthropicProvider builds a client-backed provider. apiKey must be
// non-empty; callers should fall back to StubProvider otherwise.
func NewAnthropicProvider(apiKey, model string, store storage.Store) AnthropicProvider {
	return AnthropicProvider{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:  model,
		store:  store,
	}
}

const transcribeInstruction = "Transcribe every piece of text visible in this document exactly as written, " +
	"including field labels and their values (e.g. \"Vendor: Acme Corp\"). Preserve line breaks between " +
	"fields. Output only the transcription — no commentary, no markdown formatting."

// ExtractText sends the stored file's bytes to Claude as a PDF or image
// content block (per mimeType) and returns its transcription. mimeType
// is expected to be one of the types internal/api.ValidateUpload
// allows (application/pdf, image/png, image/jpeg) — anything else is
// an error rather than a guess.
func (p AnthropicProvider) ExtractText(ctx context.Context, path, mimeType string) (string, error) {
	if !isSupportedMimeType(mimeType) {
		return "", fmt.Errorf("anthropic OCR: unsupported mime type %q", mimeType)
	}

	f, err := p.store.Open(ctx, path)
	if err != nil {
		return "", fmt.Errorf("anthropic OCR: open document: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("anthropic OCR: read document: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)

	var contentBlock anthropic.ContentBlockParamUnion
	switch mimeType {
	case "application/pdf":
		contentBlock = anthropic.NewDocumentBlock(anthropic.Base64PDFSourceParam{Data: encoded})
	default: // "image/png", "image/jpeg" — the only other case isSupportedMimeType allows
		contentBlock = anthropic.NewImageBlockBase64(mimeType, encoded)
	}

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 4096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(contentBlock, anthropic.NewTextBlock(transcribeInstruction)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("anthropic OCR: %w", err)
	}

	var text strings.Builder
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(tb.Text)
		}
	}
	if text.Len() == 0 {
		return "", fmt.Errorf("anthropic OCR: no text in response (stop_reason=%s)", resp.StopReason)
	}
	return text.String(), nil
}

// isSupportedMimeType matches the set internal/api.ValidateUpload
// allows through the upload endpoint — kept as an explicit allowlist
// here too rather than trusting the caller, since a wrong content-block
// type for the actual bytes would be a confusing failure deep in the
// Anthropic API response instead of a clear error at the call site.
func isSupportedMimeType(mimeType string) bool {
	switch mimeType {
	case "application/pdf", "image/png", "image/jpeg":
		return true
	default:
		return false
	}
}
