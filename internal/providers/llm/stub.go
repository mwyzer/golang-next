package llm

import (
	"context"
	"strconv"
	"strings"

	"golang-nextjs/internal/domain"
)

// StubProvider classifies by keyword matching instead of calling a real
// LLM. It exists so the agent pipeline is runnable end-to-end before an
// LLM provider is chosen (see PRD Open Questions).
type StubProvider struct{}

func (StubProvider) Classify(_ context.Context, text string) (ClassifyResult, error) {
	lower := strings.ToLower(text)

	switch {
	case strings.Contains(lower, "invoice"):
		return ClassifyResult{DocumentType: domain.DocumentTypeInvoice, Confidence: 0.95}, nil
	case strings.Contains(lower, "receipt"):
		return ClassifyResult{DocumentType: domain.DocumentTypeReceipt, Confidence: 0.95}, nil
	case containsAny(lower, "resume", "curriculum vitae", " cv ", "candidate"):
		return ClassifyResult{DocumentType: domain.DocumentTypeCV, Confidence: 0.9}, nil
	default:
		return ClassifyResult{DocumentType: domain.DocumentTypeUnknown, Confidence: 0.2}, nil
	}
}

// Extract is a line-based "Label: value" heuristic, not a real LLM call.
// It's a placeholder for the extract_fields tool until an LLM provider
// is chosen (see PRD Open Questions) — good enough to exercise the
// pipeline end-to-end, not to extract from real-world documents.
func (StubProvider) Extract(_ context.Context, text string, schema map[string]string) (ExtractResult, error) {
	labels := parseLabelledLines(text)
	fields := make(map[string]FieldExtraction, len(schema))

	for fieldName, fieldType := range schema {
		fieldKey := strings.ToLower(strings.ReplaceAll(fieldName, "_", " "))

		var (
			rawValue string
			found    bool
		)
		for label, value := range labels {
			if strings.Contains(fieldKey, label) {
				rawValue, found = value, true
				break
			}
		}

		if !found {
			fields[fieldName] = FieldExtraction{Value: nil, Confidence: 0}
			continue
		}

		fields[fieldName] = convertField(rawValue, fieldType)
	}

	return ExtractResult{Fields: fields}, nil
}

// parseLabelledLines splits text into "label: value" pairs, keyed by
// the lowercased label.
func parseLabelledLines(text string) map[string]string {
	labels := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		label, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		label = strings.ToLower(strings.TrimSpace(label))
		value = strings.TrimSpace(value)
		if label == "" || value == "" {
			continue
		}
		labels[label] = value
	}
	return labels
}

func convertField(rawValue, fieldType string) FieldExtraction {
	switch fieldType {
	case "number":
		if n, err := strconv.ParseFloat(strings.TrimSpace(rawValue), 64); err == nil {
			return FieldExtraction{Value: n, Confidence: 0.9}
		}
		// Label matched but the value didn't parse cleanly as a number.
		return FieldExtraction{Value: rawValue, Confidence: 0.5}
	case "array":
		parts := strings.Split(rawValue, ",")
		items := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				items = append(items, p)
			}
		}
		return FieldExtraction{Value: items, Confidence: 0.9}
	default: // "string", "date"
		return FieldExtraction{Value: rawValue, Confidence: 0.9}
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
