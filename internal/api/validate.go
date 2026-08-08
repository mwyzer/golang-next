package api

import (
	"golang-nextjs/internal/domain"
)

var allowedMimeTypes = map[string]bool{
	"application/pdf": true,
	"image/png":       true,
	"image/jpeg":      true,
}

// ValidateUpload checks the uploaded file's MIME type and size against
// the constraints from docs/SRS.md (Feature: Document Upload).
func ValidateUpload(mimeType string, sizeBytes, maxBytes int64) error {
	if !allowedMimeTypes[mimeType] {
		return domain.ErrUnsupportedFileType
	}
	if sizeBytes > maxBytes {
		return domain.ErrFileTooLarge
	}
	return nil
}
