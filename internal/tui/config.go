package tui

import (
	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
)

// Config holds TUI configuration.
type Config struct {
	Theme                  themes.Theme
	Storage                service.Storage
	Classifier             engine.Classifier
	TestData               TestDataConfig
	Width                  int
	Height                 int
	ShowStats              bool
	ShowPreview            bool
	EnableVirtualScrolling bool
	EnableCaching          bool
	EnableAnimations       bool
	MouseSupport           bool
	TestMode               bool
	ShowHelp               bool
}

// TestDataConfig holds configuration for test data generation.
type TestDataConfig struct {
	MerchantNames    []string
	TransactionCount int
	CategoryCount    int
}

// Option is a functional option for configuring the TUI.
type Option func(*Config)

// defaultConfig returns the default configuration.
func defaultConfig() Config {
	return Config{
		Theme:                  themes.Default,
		Width:                  80,
		Height:                 24,
		ShowHelp:               true,
		ShowStats:              true,
		ShowPreview:            true,
		EnableVirtualScrolling: true,
		EnableCaching:          true,
		EnableAnimations:       true,
		MouseSupport:           true,
		TestMode:               false,
		TestData: TestDataConfig{
			TransactionCount: 100,
			CategoryCount:    20,
			MerchantNames: []string{
				"Whole Foods Market",
				"Amazon.com",
				"Shell Oil",
				"Netflix",
				"Starbucks",
				"Target",
				"Uber",
				"Spotify",
				"CVS Pharmacy",
				"Home Depot",
			},
		},
	}
}

// WithStorage sets the storage service.
func WithStorage(storage service.Storage) Option {
	return func(c *Config) {
		c.Storage = storage
	}
}

// WithClassifier sets the AI classifier.
func WithClassifier(classifier engine.Classifier) Option {
	return func(c *Config) {
		c.Classifier = classifier
	}
}

// WithTheme sets the visual theme.
func WithTheme(theme themes.Theme) Option {
	return func(c *Config) {
		c.Theme = theme
	}
}

// WithSize sets the initial terminal size.
func WithSize(width, height int) Option {
	return func(c *Config) {
		c.Width = width
		c.Height = height
	}
}

// WithTestMode enables test mode with fake data.
func WithTestMode(enabled bool) Option {
	return func(c *Config) {
		c.TestMode = enabled
	}
}

// WithFeatures configures UI features.
func WithFeatures(virtual, cache, animations, mouse bool) Option {
	return func(c *Config) {
		c.EnableVirtualScrolling = virtual
		c.EnableCaching = cache
		c.EnableAnimations = animations
		c.MouseSupport = mouse
	}
}

// WithTestData configures test data generation.
func WithTestData(count int, merchants []string) Option {
	return func(c *Config) {
		c.TestData.TransactionCount = count
		c.TestData.MerchantNames = merchants
	}
}
