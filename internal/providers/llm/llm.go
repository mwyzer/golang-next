// Package llm abstracts the classification/extraction/reasoning calls
// made by the AI Agent so the provider (OpenAI, Anthropic, ...) can be
// swapped without touching agent code (NFR-17). The PRD leaves the
// concrete provider as an open question; StubProvider stands in until
// one is chosen.
package llm

import "context"

type ClassifyResult struct {
	DocumentType string
	Confidence   float64
}

type Provider interface {
	// Classify predicts the document type for the given text, matching
	// the classify_document tool in docs/architecture/agent-architecture.md.
	Classify(ctx context.Context, text string) (ClassifyResult, error)
}
