package ocr

import (
	"context"
	"fmt"
	"io"

	"golang-nextjs/internal/storage"
)

// StubProvider reads the stored file's raw bytes as text via the
// storage.Store abstraction. It does not perform real OCR on images or
// PDFs — it exists so the classify/extract pipeline can be built and
// tested before a real OCR engine is wired in (see PRD Open Questions).
type StubProvider struct {
	Store storage.Store
}

func (p StubProvider) ExtractText(ctx context.Context, path, _ string) (string, error) {
	f, err := p.Store.Open(ctx, path)
	if err != nil {
		return "", fmt.Errorf("open document: %w", err)
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("read document: %w", err)
	}
	return string(b), nil
}
