package llm

import (
	"context"
	"testing"

	"golang-nextjs/internal/domain"
)

func TestStubProviderClassify(t *testing.T) {
	p := StubProvider{}

	tests := []struct {
		name     string
		text     string
		wantType string
	}{
		{"invoice keyword", "This is an INVOICE for services rendered.", domain.DocumentTypeInvoice},
		{"receipt keyword", "Store Receipt - Thank you for shopping.", domain.DocumentTypeReceipt},
		{"resume keyword", "John Doe - Resume\nSoftware Engineer", domain.DocumentTypeCV},
		{"no match", "Lorem ipsum dolor sit amet.", domain.DocumentTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Classify(context.Background(), tt.text)
			if err != nil {
				t.Fatalf("Classify() error = %v", err)
			}
			if result.DocumentType != tt.wantType {
				t.Fatalf("Classify(%q) = %q, want %q", tt.text, result.DocumentType, tt.wantType)
			}
		})
	}
}
