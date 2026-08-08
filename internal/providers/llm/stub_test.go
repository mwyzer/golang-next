package llm

import (
	"context"
	"reflect"
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

func TestStubProviderExtract(t *testing.T) {
	p := StubProvider{}
	schema := map[string]string{
		"vendor_name":  "string",
		"total_amount": "number",
		"invoice_date": "date",
	}
	text := "INVOICE\nVendor: Acme Corp\nTotal: 129.99\nInvoice Date: 2026-08-09"

	result, err := p.Extract(context.Background(), text, schema)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	vendor := result.Fields["vendor_name"]
	if vendor.Value != "Acme Corp" || vendor.Confidence == 0 {
		t.Errorf("vendor_name = %+v, want value %q with confidence > 0", vendor, "Acme Corp")
	}

	total := result.Fields["total_amount"]
	if !reflect.DeepEqual(total.Value, 129.99) {
		t.Errorf("total_amount = %+v, want numeric value 129.99", total)
	}

	date := result.Fields["invoice_date"]
	if date.Value != "2026-08-09" {
		t.Errorf("invoice_date = %+v, want value %q", date, "2026-08-09")
	}
}

func TestStubProviderExtractMissingField(t *testing.T) {
	p := StubProvider{}
	schema := map[string]string{"vendor_name": "string"}

	result, err := p.Extract(context.Background(), "no relevant content here", schema)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}

	vendor := result.Fields["vendor_name"]
	if vendor.Value != nil || vendor.Confidence != 0 {
		t.Errorf("vendor_name = %+v, want nil value and zero confidence for a missing field", vendor)
	}
}
