package llm

import (
	"encoding/json"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClamp01(t *testing.T) {
	assert.Equal(t, 0.0, clamp01(-1))
	assert.Equal(t, 1.0, clamp01(2))
	assert.Equal(t, 0.42, clamp01(0.42))
}

func TestIsValidDocumentType(t *testing.T) {
	assert.True(t, isValidDocumentType("invoice"))
	assert.True(t, isValidDocumentType("unknown"))
	assert.False(t, isValidDocumentType("passport"))
	assert.False(t, isValidDocumentType(""))
}

func TestValueSchemaFor(t *testing.T) {
	assert.Equal(t, []string{"number", "null"}, valueSchemaFor("number")["type"])
	assert.Equal(t, []string{"array", "null"}, valueSchemaFor("array")["type"])
	assert.Equal(t, []string{"string", "null"}, valueSchemaFor("string")["type"])
	assert.Equal(t, []string{"string", "null"}, valueSchemaFor("date")["type"])
}

// newMessageWithToolUse builds an anthropic.Message from a JSON fixture
// so extractToolInput can be exercised without a live API call.
func newMessageWithToolUse(t *testing.T, toolName string, input any) *anthropic.Message {
	t.Helper()
	inputJSON, err := json.Marshal(input)
	require.NoError(t, err)

	fixture := map[string]any{
		"id":          "msg_test",
		"type":        "message",
		"role":        "assistant",
		"model":       "claude-opus-5",
		"stop_reason": "tool_use",
		"usage":       map[string]any{"input_tokens": 1, "output_tokens": 1},
		"content": []map[string]any{
			{
				"type":  "tool_use",
				"id":    "toolu_test",
				"name":  toolName,
				"input": json.RawMessage(inputJSON),
			},
		},
	}
	raw, err := json.Marshal(fixture)
	require.NoError(t, err)

	var msg anthropic.Message
	require.NoError(t, json.Unmarshal(raw, &msg))
	return &msg
}

func TestExtractToolInput_Success(t *testing.T) {
	msg := newMessageWithToolUse(t, "classify_document", classifyToolInput{
		DocumentType: "invoice",
		Confidence:   0.9,
	})

	var out classifyToolInput
	err := extractToolInput(msg, "classify_document", &out)

	require.NoError(t, err)
	assert.Equal(t, "invoice", out.DocumentType)
	assert.Equal(t, 0.9, out.Confidence)
}

func TestExtractToolInput_WrongToolName(t *testing.T) {
	msg := newMessageWithToolUse(t, "some_other_tool", classifyToolInput{DocumentType: "invoice", Confidence: 0.9})

	var out classifyToolInput
	err := extractToolInput(msg, "classify_document", &out)

	assert.Error(t, err)
}

func TestExtractToolInput_NoContent(t *testing.T) {
	msg := &anthropic.Message{StopReason: anthropic.StopReasonMaxTokens}

	var out classifyToolInput
	err := extractToolInput(msg, "classify_document", &out)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "classify_document")
}
