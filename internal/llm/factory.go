package llm

import (
	"fmt"
	"strings"
)

// NewClient creates a raw LLM client based on the provided configuration.
// This is primarily used for analysis operations that need direct access to the LLM.
func NewClient(cfg Config) (Client, error) {
	switch strings.ToLower(cfg.Provider) {
	case "openai":
		return newOpenAIClient(cfg)
	case "anthropic":
		return newAnthropicClient(cfg)
	case "claudecode":
		return newClaudeCodeClient(cfg)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", cfg.Provider)
	}
}
