package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"golang-nextjs/internal/domain"
)

// AnthropicProvider implements Provider using the Claude Messages API
// (github.com/anthropics/anthropic-sdk-go), forcing a single tool call
// per request so the response is structured JSON rather than free text.
// It's the real LLM backing classify_document/extract_fields once
// ANTHROPIC_API_KEY is configured (internal/config); until then
// Runner.LLM stays on StubProvider — see NewProvider in this package.
type AnthropicProvider struct {
	client anthropic.Client
	model  string
}

// NewAnthropicProvider builds a client-backed provider. apiKey must be
// non-empty; callers should fall back to StubProvider otherwise.
func NewAnthropicProvider(apiKey, model string) AnthropicProvider {
	return AnthropicProvider{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:  model,
	}
}

var validDocumentTypes = []string{
	domain.DocumentTypeInvoice,
	domain.DocumentTypeReceipt,
	domain.DocumentTypeCV,
	domain.DocumentTypeUnknown,
}

type classifyToolInput struct {
	DocumentType string  `json:"document_type"`
	Confidence   float64 `json:"confidence"`
}

func (p AnthropicProvider) Classify(ctx context.Context, text string) (ClassifyResult, error) {
	enumValues := make([]any, len(validDocumentTypes))
	for i, t := range validDocumentTypes {
		enumValues[i] = t
	}

	tool := anthropic.ToolParam{
		Name:        "classify_document",
		Description: anthropic.String("Record the predicted document type and confidence."),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: map[string]any{
				"document_type": map[string]any{
					"type": "string",
					"enum": enumValues,
				},
				"confidence": map[string]any{
					"type":        "number",
					"description": "Confidence in the classification, from 0 to 1.",
				},
			},
			Required: []string{"document_type", "confidence"},
		},
	}

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{{
			Text: "You classify business documents from their extracted text. Call classify_document exactly " +
				"once with your best guess. Use \"unknown\" when the type doesn't clearly match one of the " +
				"other allowed values.",
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(text)),
		},
		Tools:      []anthropic.ToolUnionParam{{OfTool: &tool}},
		ToolChoice: anthropic.ToolChoiceUnionParam{OfTool: &anthropic.ToolChoiceToolParam{Name: "classify_document"}},
	})
	if err != nil {
		return ClassifyResult{}, fmt.Errorf("anthropic classify: %w", err)
	}

	var out classifyToolInput
	if err := extractToolInput(resp, "classify_document", &out); err != nil {
		return ClassifyResult{}, fmt.Errorf("anthropic classify: %w", err)
	}

	docType := strings.ToLower(strings.TrimSpace(out.DocumentType))
	if !isValidDocumentType(docType) {
		docType = domain.DocumentTypeUnknown
	}

	return ClassifyResult{DocumentType: docType, Confidence: clamp01(out.Confidence)}, nil
}

// Extract asks Claude to fill in one tool call whose schema mirrors
// schema (field name -> "string"/"number"/"date"/"array"), then maps
// the result back onto ExtractResult. Fields the model can't find are
// returned with Value: nil, Confidence: 0, matching StubProvider so
// downstream code (validate_extraction, calculate_confidence) doesn't
// need to special-case which provider ran.
func (p AnthropicProvider) Extract(ctx context.Context, text string, schema map[string]string) (ExtractResult, error) {
	properties := make(map[string]any, len(schema))
	required := make([]string, 0, len(schema))
	for name, fieldType := range schema {
		properties[name] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"value":      valueSchemaFor(fieldType),
				"confidence": map[string]any{"type": "number", "description": "0 to 1; 0 if the field wasn't found."},
			},
			"required": []string{"value", "confidence"},
		}
		required = append(required, name)
	}

	tool := anthropic.ToolParam{
		Name:        "extract_fields",
		Description: anthropic.String("Record the extracted value and confidence for every requested field."),
		InputSchema: anthropic.ToolInputSchemaParam{
			Properties: properties,
			Required:   required,
		},
	}

	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: 2048,
		System: []anthropic.TextBlockParam{{
			Text: "You extract structured fields from a business document's text. Call extract_fields exactly " +
				"once. For a field you cannot find in the text, set its value to null and confidence to 0 " +
				"rather than guessing.",
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(text)),
		},
		Tools:      []anthropic.ToolUnionParam{{OfTool: &tool}},
		ToolChoice: anthropic.ToolChoiceUnionParam{OfTool: &anthropic.ToolChoiceToolParam{Name: "extract_fields"}},
	})
	if err != nil {
		return ExtractResult{}, fmt.Errorf("anthropic extract: %w", err)
	}

	var raw map[string]struct {
		Value      any     `json:"value"`
		Confidence float64 `json:"confidence"`
	}
	if err := extractToolInput(resp, "extract_fields", &raw); err != nil {
		return ExtractResult{}, fmt.Errorf("anthropic extract: %w", err)
	}

	fields := make(map[string]FieldExtraction, len(schema))
	for name := range schema {
		if v, ok := raw[name]; ok {
			fields[name] = FieldExtraction{Value: v.Value, Confidence: clamp01(v.Confidence)}
		} else {
			fields[name] = FieldExtraction{Value: nil, Confidence: 0}
		}
	}

	return ExtractResult{Fields: fields}, nil
}

// valueSchemaFor maps an extract_fields schema type (as used throughout
// internal/agent) to the JSON schema for that field's "value" property.
// Nullable so the model can express "not found" without inventing a
// zero value.
func valueSchemaFor(fieldType string) map[string]any {
	switch fieldType {
	case "number":
		return map[string]any{"type": []string{"number", "null"}}
	case "array":
		return map[string]any{"type": []string{"array", "null"}, "items": map[string]any{"type": "string"}}
	default: // "string", "date"
		return map[string]any{"type": []string{"string", "null"}}
	}
}

func isValidDocumentType(docType string) bool {
	for _, t := range validDocumentTypes {
		if t == docType {
			return true
		}
	}
	return false
}

func clamp01(f float64) float64 {
	switch {
	case f < 0:
		return 0
	case f > 1:
		return 1
	default:
		return f
	}
}

// extractToolInput finds the first tool_use block named toolName in
// resp and unmarshals its input into out. Forcing ToolChoice to that
// tool name makes a miss unlikely, but a defensive error beats a nil
// pointer deref if the model ever stops early (e.g. hitting max_tokens).
func extractToolInput(resp *anthropic.Message, toolName string, out any) error {
	for _, block := range resp.Content {
		if tu, ok := block.AsAny().(anthropic.ToolUseBlock); ok && tu.Name == toolName {
			return json.Unmarshal(tu.Input, out)
		}
	}
	return fmt.Errorf("no %s tool_use block in response (stop_reason=%s)", toolName, resp.StopReason)
}
