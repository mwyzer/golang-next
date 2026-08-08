package domain

import "time"

type ExtractedFieldSource string

const (
	ExtractedFieldSourceExtraction       ExtractedFieldSource = "extraction"
	ExtractedFieldSourceReviewCorrection ExtractedFieldSource = "review_correction"
)

// ExtractedField is a single field pulled from a document by the
// extract_fields tool (SRS Feature: Structured Extraction). A field with
// Value == nil and Confidence == 0 means the schema expected it but the
// extractor couldn't find it — it's still recorded, not dropped, so
// missing required fields stay visible (SRS acceptance criteria).
type ExtractedField struct {
	ID         string
	DocumentID string
	FieldName  string
	Value      any
	Confidence float64
	Source     ExtractedFieldSource
	CreatedAt  time.Time
}
