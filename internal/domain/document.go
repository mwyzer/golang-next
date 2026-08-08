package domain

import "time"

// Document type IDs, matching the seeded rows in
// db/migrations/000002_seed_dev_data.up.sql. "unknown" is not a stored
// document_types row — it signals the classifier couldn't confidently
// match one of the supported types (SRS Feature: Document Classification).
const (
	DocumentTypeInvoice = "invoice"
	DocumentTypeReceipt = "receipt"
	DocumentTypeCV      = "cv"
	DocumentTypeUnknown = "unknown"
)

type DocumentStatus string

const (
	StatusUploaded      DocumentStatus = "UPLOADED"
	StatusClassified    DocumentStatus = "CLASSIFIED"
	StatusExtracted     DocumentStatus = "EXTRACTED"
	StatusValidated     DocumentStatus = "VALIDATED"
	StatusPendingReview DocumentStatus = "PENDING_REVIEW"
	StatusAutoProcessed DocumentStatus = "AUTO_PROCESSED"
	StatusReviewed      DocumentStatus = "REVIEWED"
	StatusFailed        DocumentStatus = "FAILED"
)

// Document is an uploaded file and its processing state (docs/technical-design/db.md#documents).
type Document struct {
	ID                       string
	TenantID                 string
	UploadedBy               string
	DocumentTypeID           *string
	Status                   DocumentStatus
	FilePath                 string
	MimeType                 string
	FileSizeBytes            int64
	ContentHash              string
	ClassificationConfidence *float64
	OverallConfidence        *float64
	IsDuplicate              bool
	DuplicateOfDocumentID    *string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}
