package ocr

import (
	"fmt"

	"golang-nextjs/internal/storage"
)

// NewProvider selects a Provider by name — "stub" (default) or
// "anthropic" — mirroring llm.NewProvider (internal/providers/llm/factory.go).
func NewProvider(providerName, anthropicAPIKey, anthropicModel string, store storage.Store) (Provider, error) {
	switch providerName {
	case "", "stub":
		return StubProvider{Store: store}, nil
	case "anthropic":
		if anthropicAPIKey == "" {
			return nil, fmt.Errorf("OCR_PROVIDER=anthropic requires ANTHROPIC_API_KEY to be set")
		}
		return NewAnthropicProvider(anthropicAPIKey, anthropicModel, store), nil
	default:
		return nil, fmt.Errorf("unknown OCR_PROVIDER %q (want \"stub\" or \"anthropic\")", providerName)
	}
}
