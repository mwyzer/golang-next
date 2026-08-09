package llm

import "fmt"

// NewProvider selects a Provider by name — "stub" (default) or
// "anthropic" — following the env-var-with-fallback pattern used
// throughout internal/config. Keeping the switch here (rather than
// inline in cmd/worker) means both the worker and any future caller
// build providers the same way, and unit tests can exercise the
// unknown-provider/missing-key error paths without a live API key.
func NewProvider(providerName, anthropicAPIKey, anthropicModel string) (Provider, error) {
	switch providerName {
	case "", "stub":
		return StubProvider{}, nil
	case "anthropic":
		if anthropicAPIKey == "" {
			return nil, fmt.Errorf("LLM_PROVIDER=anthropic requires ANTHROPIC_API_KEY to be set")
		}
		return NewAnthropicProvider(anthropicAPIKey, anthropicModel), nil
	default:
		return nil, fmt.Errorf("unknown LLM_PROVIDER %q (want \"stub\" or \"anthropic\")", providerName)
	}
}
