package api

import (
	"errors"
	"testing"

	"golang-nextjs/internal/domain"
)

func TestValidateUpload(t *testing.T) {
	const maxBytes = 1000

	tests := []struct {
		name     string
		mimeType string
		size     int64
		wantErr  error
	}{
		{"valid pdf", "application/pdf", 500, nil},
		{"valid png", "image/png", 500, nil},
		{"valid jpeg", "image/jpeg", 500, nil},
		{"unsupported type", "text/plain", 500, domain.ErrUnsupportedFileType},
		{"too large", "application/pdf", 1001, domain.ErrFileTooLarge},
		{"exactly at limit", "application/pdf", maxBytes, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUpload(tt.mimeType, tt.size, maxBytes)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateUpload(%q, %d) = %v, want %v", tt.mimeType, tt.size, err, tt.wantErr)
			}
		})
	}
}
