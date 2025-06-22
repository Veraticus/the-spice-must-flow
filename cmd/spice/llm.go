package main

import (
	"fmt"
	"os"
	"time"

	"log/slog"

	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/llm"
	"github.com/spf13/viper"
)

// createLLMClient creates an LLM client based on configuration.
// This function is shared by multiple commands that need LLM functionality.
func createLLMClient() (engine.Classifier, error) {
	// Read LLM configuration from viper
	provider := viper.GetString("llm.provider")
	if provider == "" {
		provider = "openai" // default provider
	}

	// Build config from viper settings
	config := llm.Config{
		Provider:       provider,
		Model:          viper.GetString("llm.model"),
		Temperature:    viper.GetFloat64("llm.temperature"),
		MaxTokens:      viper.GetInt("llm.max_tokens"),
		MaxRetries:     viper.GetInt("llm.max_retries"),
		RetryDelay:     viper.GetDuration("llm.retry_delay"),
		CacheTTL:       viper.GetDuration("llm.cache_ttl"),
		RateLimit:      viper.GetInt("llm.rate_limit"),
		ClaudeCodePath: viper.GetString("llm.claude_code_path"),
	}

	// Set defaults if not specified
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = time.Second
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 24 * time.Hour
	}
	if config.RateLimit == 0 {
		config.RateLimit = 1000 // requests per minute
	}

	// Get API key based on provider
	switch provider {
	case "openai":
		// Check viper first, then environment variable
		apiKey := viper.GetString("llm.openai_api_key")
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("OpenAI API key not found in config or OPENAI_API_KEY environment variable")
		}
		config.APIKey = apiKey

		// Set default model if not specified
		if config.Model == "" {
			config.Model = "gpt-4-turbo-preview"
		}

	case "anthropic":
		// Check viper first, then environment variable
		apiKey := viper.GetString("llm.anthropic_api_key")
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("anthropic API key not found in config or ANTHROPIC_API_KEY environment variable")
		}
		config.APIKey = apiKey

		// Set default model if not specified
		if config.Model == "" {
			config.Model = "claude-3-opus-20240229"
		}

	case "claudecode":
		// Claude Code doesn't need an API key
		config.APIKey = ""

		// Set default model if not specified
		if config.Model == "" {
			config.Model = "claude-3-opus-20240229"
		}

	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}

	// Create LLM classifier
	classifier, err := llm.NewClassifier(config, slog.Default())
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM classifier: %w", err)
	}

	return classifier, nil
}
