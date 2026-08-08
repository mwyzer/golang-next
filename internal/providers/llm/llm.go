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

// FieldExtraction is one field's result from Extract. Value is nil and
// Confidence is 0 when the schema expected the field but it wasn't
// found in the text — callers must still record it, not drop it (SRS
// Feature: Structured Extraction, "missing fields are flagged").
type FieldExtraction struct {
	Value      any
	Confidence float64
}

type ExtractResult struct {
	Fields map[string]FieldExtraction
}

type Provider interface {
	// Classify predicts the document type for the given text, matching
	// the classify_document tool in docs/architecture/agent-architecture.md.
	Classify(ctx context.Context, text string) (ClassifyResult, error)

	// Extract pulls the fields named in schema (field name -> type, e.g.
	// "string"/"number"/"date"/"array") out of text, matching the
	// extract_fields tool.
	Extract(ctx context.Context, text string, schema map[string]string) (ExtractResult, error)
}
