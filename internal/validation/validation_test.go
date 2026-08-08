package validation

import "testing"

func TestValidate(t *testing.T) {
	schema := map[string]string{
		"vendor_name":  "string",
		"total_amount": "number",
		"invoice_date": "date",
	}

	tests := []struct {
		name       string
		values     map[string]any
		wantRuleID string // "" means no violations expected
	}{
		{
			name: "all valid",
			values: map[string]any{
				"vendor_name": "Acme Corp", "total_amount": 129.99, "invoice_date": "2026-08-09",
			},
			wantRuleID: "",
		},
		{
			name: "missing field",
			values: map[string]any{
				"vendor_name": "Acme Corp", "total_amount": 129.99, "invoice_date": nil,
			},
			wantRuleID: "field_required",
		},
		{
			name: "negative amount",
			values: map[string]any{
				"vendor_name": "Acme Corp", "total_amount": -5.0, "invoice_date": "2026-08-09",
			},
			wantRuleID: "field_numeric_range",
		},
		{
			name: "non-numeric amount",
			values: map[string]any{
				"vendor_name": "Acme Corp", "total_amount": "not a number", "invoice_date": "2026-08-09",
			},
			wantRuleID: "field_numeric",
		},
		{
			name: "malformed date",
			values: map[string]any{
				"vendor_name": "Acme Corp", "total_amount": 129.99, "invoice_date": "08/09/2026",
			},
			wantRuleID: "field_date_format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := Validate(schema, tt.values)

			if tt.wantRuleID == "" {
				if len(violations) != 0 {
					t.Fatalf("Validate() = %+v, want no violations", violations)
				}
				return
			}

			found := false
			for _, v := range violations {
				if v.RuleID == tt.wantRuleID {
					found = true
				}
			}
			if !found {
				t.Fatalf("Validate() = %+v, want a violation with rule ID %q", violations, tt.wantRuleID)
			}
		})
	}
}
