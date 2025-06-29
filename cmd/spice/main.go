package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	version = "dev"
	rootCmd = &cobra.Command{
		Use:   "spice",
		Short: "🌶️  Personal finance categorization engine",
		Long: `the-spice-must-flow: A delightful CLI tool that ingests financial transactions,
intelligently categorizes them, and exports clean reports for your accountant.

The spice must flow!`,
		PersistentPreRunE: initConfig,
	}
)

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: $HOME/.config/spice/config.yaml)")
	rootCmd.PersistentFlags().String("log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("log-format", "console", "log format (console, json)")

	// Bind flags to viper
	_ = viper.BindPFlag("logging.level", rootCmd.PersistentFlags().Lookup("log-level"))
	_ = viper.BindPFlag("logging.format", rootCmd.PersistentFlags().Lookup("log-format"))

	// Add commands
	rootCmd.AddCommand(analyzeCmd())
	rootCmd.AddCommand(authCmd())
	rootCmd.AddCommand(categoriesCmd())
	rootCmd.AddCommand(checkpointCmd())
	rootCmd.AddCommand(checksCmd())
	rootCmd.AddCommand(classifyCmd())
	rootCmd.AddCommand(importCmd())
	rootCmd.AddCommand(vendorsCmd())
	rootCmd.AddCommand(patternsCmd())
	rootCmd.AddCommand(flowCmd())
	rootCmd.AddCommand(migrateCmd())
	rootCmd.AddCommand(institutionsCmd())
	rootCmd.AddCommand(recategorizeCmd())
	rootCmd.AddCommand(versionCmd())
}

func main() {
	// Create a base context that commands can use
	ctx := context.Background()

	err := rootCmd.ExecuteContext(ctx)

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initConfig(_ *cobra.Command, _ []string) error {
	// Set up config file
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		// Search for config in standard locations
		viper.AddConfigPath(fmt.Sprintf("%s/.config/spice", home))
		viper.AddConfigPath(".")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Environment variables
	viper.SetEnvPrefix("SPICE")
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("failed to read config: %w", err)
		}
		// Config file not found is OK, we'll use defaults
	}

	// Set up logging
	if err := setupLogging(); err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	return nil
}

func setupLogging() error {
	level := viper.GetString("logging.level")
	format := viper.GetString("logging.format")

	// Parse log level
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "info":
		slogLevel = slog.LevelInfo
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		return fmt.Errorf("invalid log level: %s", level)
	}

	// Create handler based on format
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slogLevel,
	}

	switch format {
	case "console":
		handler = slog.NewTextHandler(os.Stderr, opts)
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, opts)
	default:
		return fmt.Errorf("invalid log format: %s", format)
	}

	// Set default logger
	slog.SetDefault(slog.New(handler))

	return nil
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(_ *cobra.Command, _ []string) {
			slog.Info("spice version", "version", version)
		},
	}
}
