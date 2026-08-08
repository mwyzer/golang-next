package domain

import "time"

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
