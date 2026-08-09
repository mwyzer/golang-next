package ocr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProvider_DefaultsToStub(t *testing.T) {
	p, err := NewProvider("", "", "", nil)
	require.NoError(t, err)
	assert.IsType(t, StubProvider{}, p)

	p, err = NewProvider("stub", "", "", nil)
	require.NoError(t, err)
	assert.IsType(t, StubProvider{}, p)
}

func TestNewProvider_AnthropicRequiresAPIKey(t *testing.T) {
	_, err := NewProvider("anthropic", "", "claude-opus-5", nil)
	assert.ErrorContains(t, err, "ANTHROPIC_API_KEY")
}

func TestNewProvider_AnthropicWithKey(t *testing.T) {
	p, err := NewProvider("anthropic", "sk-test-key", "claude-opus-5", nil)
	require.NoError(t, err)
	assert.IsType(t, AnthropicProvider{}, p)
}

func TestNewProvider_UnknownProvider(t *testing.T) {
	_, err := NewProvider("bogus", "", "", nil)
	assert.ErrorContains(t, err, "bogus")
}
