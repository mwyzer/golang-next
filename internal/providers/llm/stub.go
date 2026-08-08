package llm

import (
	"context"
	"strings"

	"golang-nextjs/internal/domain"
)

// StubProvider classifies by keyword matching instead of calling a real
// LLM. It exists so the agent pipeline is runnable end-to-end before an
// LLM provider is chosen (see PRD Open Questions).
type StubProvider struct{}

func (StubProvider) Classify(_ context.Context, text string) (ClassifyResult, error) {
	lower := strings.ToLower(text)

	switch {
	case strings.Contains(lower, "invoice"):
		return ClassifyResult{DocumentType: domain.DocumentTypeInvoice, Confidence: 0.95}, nil
	case strings.Contains(lower, "receipt"):
		return ClassifyResult{DocumentType: domain.DocumentTypeReceipt, Confidence: 0.95}, nil
	case containsAny(lower, "resume", "curriculum vitae", " cv ", "candidate"):
		return ClassifyResult{DocumentType: domain.DocumentTypeCV, Confidence: 0.9}, nil
	default:
		return ClassifyResult{DocumentType: domain.DocumentTypeUnknown, Confidence: 0.2}, nil
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
