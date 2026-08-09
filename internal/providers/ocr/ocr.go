// Package ocr abstracts text extraction from uploaded documents so the
// engine (local Tesseract, a cloud OCR API, ...) can be swapped without
// touching the agent (docs/architecture/system-architecture.md, NFR-17).
// The PRD leaves the concrete provider as an open question; StubProvider
// stands in until one is chosen.
package ocr

import "context"

type Provider interface {
	// ExtractText returns the text content of the document stored at
	// path. mimeType is the document's stored MIME type (one of the
	// types ValidateUpload allows — see internal/api/validate.go), so a
	// provider that needs to know how to decode the bytes (e.g. a
	// vision-based one choosing between a PDF vs. image content block)
	// doesn't have to re-derive it from the file.
	ExtractText(ctx context.Context, path, mimeType string) (string, error)
}
