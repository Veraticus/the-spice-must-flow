package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeCmd(t *testing.T) {
	// Always skip integration tests that make real API calls
	t.Skip("Skipping integration test - disabled to avoid API costs")

	tests := []struct {
		outputCheck   func(t *testing.T, output string)
		name          string
		errorContains string
		args          []string
		wantErr       bool
	}{
		{
			name: "default execution runs analysis",
			args: []string{},
			outputCheck: func(t *testing.T, output string) {
				t.Helper()
				// Should show analysis progress bar and results
				assert.Contains(t, output, "Analysis Report - Interactive View")
			},
		},
		{
			name: "with date range",
			args: []string{"--start-date", "2024-01-01", "--end-date", "2024-12-31"},
			outputCheck: func(t *testing.T, output string) {
				t.Helper()
				// Should complete analysis and show report
				assert.Contains(t, output, "Analysis Report - Interactive View")
			},
		},
		{
			name: "with focus option",
			args: []string{"--focus", "patterns"},
			outputCheck: func(t *testing.T, output string) {
				t.Helper()
				// Should complete analysis with pattern focus
				assert.Contains(t, output, "Analysis Report - Interactive View")
			},
		},
		{
			name:          "invalid start date format",
			args:          []string{"--start-date", "01/01/2024"},
			wantErr:       true,
			errorContains: "invalid start date format",
		},
		{
			name:          "invalid end date format",
			args:          []string{"--end-date", "2024/12/31"},
			wantErr:       true,
			errorContains: "invalid end date format",
		},
		{
			name:          "invalid focus option",
			args:          []string{"--focus", "invalid"},
			wantErr:       true,
			errorContains: "invalid focus: invalid",
		},
		{
			name: "all flags together",
			args: []string{
				"--start-date", "2024-06-01",
				"--end-date", "2024-06-30",
				"--focus", "coherence",
				"--max-issues", "100",
				"--dry-run",
				"--auto-apply",
				"--session-id", "test123",
				"--output", "json",
			},
			outputCheck: func(t *testing.T, output string) {
				t.Helper()
				// JSON output should contain report structure
				assert.Contains(t, output, "sessionId")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create command
			cmd := analyzeCmd()

			// Set args
			cmd.SetArgs(tt.args)

			// Capture output
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			// Execute
			err := cmd.Execute()

			// Check error
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			// Check output
			if tt.outputCheck != nil {
				tt.outputCheck(t, buf.String())
			}
		})
	}
}

func TestAnalyzeCmd_DefaultDates(t *testing.T) {
	// Always skip integration tests that make real API calls
	t.Skip("Skipping integration test - disabled to avoid API costs")

	// Test that default dates are set correctly when not provided
	cmd := analyzeCmd()
	cmd.SetArgs([]string{})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Check that analysis ran successfully
	assert.Contains(t, output, "Analysis Report")
}

func TestAnalyzeCmd_Help(t *testing.T) {
	cmd := analyzeCmd()
	cmd.SetArgs([]string{"--help"})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Check help content
	assert.Contains(t, output, "Analyze your transaction data")
	assert.Contains(t, output, "Examples:")
	assert.Contains(t, output, "spice analyze")
	assert.Contains(t, output, "--start-date")
	assert.Contains(t, output, "--end-date")
	assert.Contains(t, output, "--focus")
	assert.Contains(t, output, "--dry-run")
	assert.Contains(t, output, "--auto-apply")
	assert.Contains(t, output, "--session-id")
}

func TestAnalyzeCmd_Interruption(t *testing.T) {
	// Skip this test as it requires full database setup
	t.Skip("Skipping interruption test - requires full database setup")
}

func TestAnalyzeCmd_FocusOptions(t *testing.T) {
	// Always skip integration tests that make real API calls
	t.Skip("Skipping integration test - disabled to avoid API costs")

	validFocusOptions := []string{"all", "coherence", "patterns", "categories"}

	for _, focus := range validFocusOptions {
		t.Run("focus_"+focus, func(t *testing.T) {
			cmd := analyzeCmd()
			cmd.SetArgs([]string{"--focus", focus})

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err := cmd.Execute()
			assert.NoError(t, err)

			output := buf.String()
			assert.Contains(t, output, "Analysis Report")
		})
	}
}

func TestAnalyzeCmd_InvalidDateCombinations(t *testing.T) {
	// Skip this test as it makes real API calls
	t.Skip("Skipping date validation test - makes real API calls")
}

// Integration test placeholder - would require full analysis engine setup.
func TestAnalyzeCmd_FullIntegration(t *testing.T) {
	t.Skip("Full integration test requires complete analysis engine implementation")

	// This test would:
	// 1. Set up a test database with sample transactions
	// 2. Create mock LLM client
	// 3. Run analysis with various options
	// 4. Verify the analysis report is generated correctly
	// 5. Test fix application with --auto-apply
	// 6. Verify session continuation works
	// 7. Test different output formats
}

// Benchmark for command parsing.
func BenchmarkAnalyzeCmd_Parse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cmd := analyzeCmd()
		cmd.SetArgs([]string{
			"--start-date", "2024-01-01",
			"--end-date", "2024-12-31",
			"--focus", "patterns",
			"--max-issues", "100",
			"--dry-run",
		})

		// Just parse, don't execute
		_ = cmd.ParseFlags([]string{
			"--start-date", "2024-01-01",
			"--end-date", "2024-12-31",
			"--focus", "patterns",
			"--max-issues", "100",
			"--dry-run",
		})
	}
}

// Test flag parsing edge cases.
func TestAnalyzeCmd_FlagParsing(t *testing.T) {
	tests := []struct {
		expected  any
		name      string
		checkFlag string
		flagType  string
		args      []string
	}{
		{
			name:      "max-issues default",
			args:      []string{},
			checkFlag: "max-issues",
			expected:  50,
			flagType:  "int",
		},
		{
			name:      "max-issues custom",
			args:      []string{"--max-issues", "200"},
			checkFlag: "max-issues",
			expected:  200,
			flagType:  "int",
		},
		{
			name:      "dry-run default",
			args:      []string{},
			checkFlag: "dry-run",
			expected:  false,
			flagType:  "bool",
		},
		{
			name:      "dry-run enabled",
			args:      []string{"--dry-run"},
			checkFlag: "dry-run",
			expected:  true,
			flagType:  "bool",
		},
		{
			name:      "output default",
			args:      []string{},
			checkFlag: "output",
			expected:  "interactive",
			flagType:  "string",
		},
		{
			name:      "output json",
			args:      []string{"--output", "json"},
			checkFlag: "output",
			expected:  "json",
			flagType:  "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := analyzeCmd()
			cmd.SetArgs(tt.args)

			// Parse flags without executing
			err := cmd.ParseFlags(tt.args)
			require.NoError(t, err)

			// Check flag value
			switch tt.flagType {
			case "int":
				val, err := cmd.Flags().GetInt(tt.checkFlag)
				require.NoError(t, err)
				assert.Equal(t, tt.expected, val)
			case "bool":
				val, err := cmd.Flags().GetBool(tt.checkFlag)
				require.NoError(t, err)
				assert.Equal(t, tt.expected, val)
			case "string":
				val, err := cmd.Flags().GetString(tt.checkFlag)
				require.NoError(t, err)
				assert.Equal(t, tt.expected, val)
			}
		})
	}
}

// Test that all documented examples work.
func TestAnalyzeCmd_Examples(t *testing.T) {
	// Always skip integration tests that make real API calls
	t.Skip("Skipping integration test - disabled to avoid API costs")

	examples := []struct {
		name string
		args []string
	}{
		{
			name: "analyze last 30 days",
			args: []string{},
		},
		{
			name: "analyze specific date range",
			args: []string{"--start-date", "2024-01-01", "--end-date", "2024-03-31"},
		},
		{
			name: "focus on patterns",
			args: []string{"--focus", "patterns"},
		},
		{
			name: "dry run",
			args: []string{"--dry-run"},
		},
		{
			name: "auto apply",
			args: []string{"--auto-apply"},
		},
		{
			name: "continue session",
			args: []string{"--session-id", "abc123"},
		},
	}

	for _, ex := range examples {
		t.Run(ex.name, func(t *testing.T) {
			cmd := analyzeCmd()
			cmd.SetArgs(ex.args)

			err := cmd.Execute()
			assert.NoError(t, err, "Example command should not error")
		})
	}
}

// Test output format validation once analysis is implemented.
func TestAnalyzeCmd_OutputFormats(t *testing.T) {
	outputFormats := []string{"interactive", "summary", "json"}

	for _, format := range outputFormats {
		t.Run("output_"+format, func(t *testing.T) {
			cmd := analyzeCmd()
			cmd.SetArgs([]string{"--output", format})

			// Currently just verifying the flag is accepted
			err := cmd.ParseFlags([]string{"--output", format})
			assert.NoError(t, err)

			val, _ := cmd.Flags().GetString("output")
			assert.Equal(t, format, val)
		})
	}
}

// Test that analyze runs without errors for valid configs.
func TestAnalyzeCmd_LogOutput(t *testing.T) {
	// Always skip integration tests that make real API calls
	t.Skip("Skipping integration test - disabled to avoid API costs")

	cmd := analyzeCmd()
	cmd.SetArgs([]string{"--focus", "patterns"})

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// Just verify we got some output
	assert.NotEmpty(t, output, "Should produce output")
}
