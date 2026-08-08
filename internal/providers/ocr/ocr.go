// Package ocr abstracts text extraction from uploaded documents so the
// engine (local Tesseract, a cloud OCR API, ...) can be swapped without
// touching the agent (docs/architecture/system-architecture.md, NFR-17).
// The PRD leaves the concrete provider as an open question; StubProvider
// stands in until one is chosen.
package ocr

import "context"

type Provider interface {
	// ExtractText returns the text content of the document stored at path.
	ExtractText(ctx context.Context, path string) (string, error)
}
