// Package validation implements the validate_extraction tool (SRS
// Feature: Validation, FR-9): checking extracted field values against
// the rules implied by the document type's field schema. Unlike
// classification/extraction this is deterministic business logic, not
// an LLM call, so it lives outside internal/providers/llm.
package validation

import (
	"fmt"
	"regexp"
)

type Violation struct {
	RuleID  string `json:"rule_id"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

var dateFormat = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// Validate checks values (field name -> extracted value) against
// schema (field name -> type). Every field in schema is required in
// this MVP (see docs/SRS.md, Structured Extraction); "number" and
// "date" fields get an extra type/range check when present.
func Validate(schema map[string]string, values map[string]any) []Violation {
	var violations []Violation

	for name, fieldType := range schema {
		value, present := values[name]
		if !present || value == nil {
			violations = append(violations, Violation{
				RuleID:  "field_required",
				Field:   name,
				Message: fmt.Sprintf("required field %q is missing", name),
			})
			continue
		}

		switch fieldType {
		case "number":
			n, isNumber := value.(float64)
			if !isNumber {
				violations = append(violations, Violation{
					RuleID:  "field_numeric",
					Field:   name,
					Message: fmt.Sprintf("field %q must be numeric", name),
				})
			} else if n < 0 {
				violations = append(violations, Violation{
					RuleID:  "field_numeric_range",
					Field:   name,
					Message: fmt.Sprintf("field %q must not be negative", name),
				})
			}
		case "date":
			s, isString := value.(string)
			if !isString || !dateFormat.MatchString(s) {
				violations = append(violations, Violation{
					RuleID:  "field_date_format",
					Field:   name,
					Message: fmt.Sprintf("field %q must be in YYYY-MM-DD format", name),
				})
			}
		}
	}

	return violations
}
