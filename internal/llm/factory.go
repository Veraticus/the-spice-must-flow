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

// NewSessionClient creates a session-capable LLM client if the provider supports it.
// Returns nil if the provider doesn't support sessions.
func NewSessionClient(cfg Config) (SessionClient, error) {
	switch strings.ToLower(cfg.Provider) {
	case "claudecode":
		// Claude Code supports sessions
		client, err := newClaudeCodeClient(cfg)
		if err != nil {
			return nil, err
		}
		// We know claudeCodeClient implements SessionClient
		if sessionClient, ok := client.(SessionClient); ok {
			return sessionClient, nil
		}
		return nil, fmt.Errorf("claude code client does not implement SessionClient")
	default:
		// Other providers don't support sessions yet
		return nil, nil
	}
}
