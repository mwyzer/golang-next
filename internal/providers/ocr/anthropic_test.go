package ocr

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSupportedMimeType(t *testing.T) {
	assert.True(t, isSupportedMimeType("application/pdf"))
	assert.True(t, isSupportedMimeType("image/png"))
	assert.True(t, isSupportedMimeType("image/jpeg"))
	assert.False(t, isSupportedMimeType("text/plain"))
	assert.False(t, isSupportedMimeType(""))
}

// TestExtractText_UnsupportedMimeType checks the mime-type allowlist is
// enforced before any store I/O — a nil store here would panic if the
// code tried to open it, so this also pins that ordering.
func TestExtractText_UnsupportedMimeType(t *testing.T) {
	p := AnthropicProvider{store: nil}

	_, err := p.ExtractText(context.Background(), "some/path", "text/plain")

	assert.ErrorContains(t, err, "unsupported mime type")
}
