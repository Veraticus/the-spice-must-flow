package analysis

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// TemplatePromptBuilder handles generation of prompts for LLM analysis using templates.
type TemplatePromptBuilder struct {
	templates map[string]*template.Template
}

// NewTemplatePromptBuilder creates a new TemplatePromptBuilder with loaded templates.
func NewTemplatePromptBuilder() (*TemplatePromptBuilder, error) {
	pb := &TemplatePromptBuilder{
		templates: make(map[string]*template.Template),
	}

	// Define template functions
	funcMap := template.FuncMap{
		"formatAmount": formatAmount,
		"formatDate":   formatDate,
		"truncate":     truncate,
		"join":         strings.Join,
	}

	// Load all templates
	templates := []string{
		"analysis_prompt",
		"json_schema",
		"correction_prompt",
	}

	for _, name := range templates {
		filename := fmt.Sprintf("templates/%s.tmpl", name)
		tmpl, err := template.New(fmt.Sprintf("%s.tmpl", name)).Funcs(funcMap).ParseFS(templateFS, filename)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", name, err)
		}
		pb.templates[name] = tmpl
	}

	return pb, nil
}

// PromptData contains all data needed for the analysis prompt.
type PromptData struct {
	DateRange       DateRange
	FileBasedData   *FileBasedPromptData
	Transactions    []model.Transaction
	Categories      []model.Category
	Patterns        []model.PatternRule
	CheckPatterns   []model.CheckPattern
	RecentVendors   []RecentVendor
	AnalysisOptions Options
	TotalCount      int
	SampleSize      int
}

// FileBasedPromptData contains information for file-based analysis.
type FileBasedPromptData struct {
	FilePath           string
	TransactionCount   int
	UseFileBasedPrompt bool
}

// RecentVendor represents a recently categorized vendor for context.
type RecentVendor struct {
	Name        string
	Category    string
	Occurrences int
}

// DateRange represents the date range being analyzed.
type DateRange struct {
	Start time.Time
	End   time.Time
}

// BuildAnalysisPrompt creates the main analysis prompt.
func (pb *TemplatePromptBuilder) BuildAnalysisPrompt(data PromptData) (string, error) {
	// First, get the JSON schema
	schemaData := struct {
		IncludePatternSuggestions bool
		IncludeCategoryAnalysis   bool
	}{
		IncludePatternSuggestions: data.AnalysisOptions.Focus == "" || data.AnalysisOptions.Focus == FocusPatterns,
		IncludeCategoryAnalysis:   data.AnalysisOptions.Focus == "" || data.AnalysisOptions.Focus == FocusCategories,
	}

	var schemaBuf bytes.Buffer
	if err := pb.templates["json_schema"].ExecuteTemplate(&schemaBuf, "json_schema.tmpl", schemaData); err != nil {
		return "", fmt.Errorf("failed to execute json_schema template: %w", err)
	}

	// Add the schema to the prompt data
	type promptDataWithSchema struct {
		JSONSchema string
		PromptData
	}

	fullData := promptDataWithSchema{
		PromptData: data,
		JSONSchema: schemaBuf.String(),
	}

	// Execute the main prompt template
	var buf bytes.Buffer
	if err := pb.templates["analysis_prompt"].ExecuteTemplate(&buf, "analysis_prompt.tmpl", fullData); err != nil {
		return "", fmt.Errorf("failed to execute analysis_prompt template: %w", err)
	}

	return buf.String(), nil
}

// CorrectionPromptData contains data for correction prompts.
type CorrectionPromptData struct {
	OriginalPrompt  string
	InvalidResponse string
	ErrorDetails    string
	ErrorSection    string
	LineNumber      int
	ColumnNumber    int
}

// BuildCorrectionPrompt creates a prompt to fix invalid JSON responses.
func (pb *TemplatePromptBuilder) BuildCorrectionPrompt(data CorrectionPromptData) (string, error) {
	var buf bytes.Buffer
	if err := pb.templates["correction_prompt"].ExecuteTemplate(&buf, "correction_prompt.tmpl", data); err != nil {
		return "", fmt.Errorf("failed to execute correction_prompt template: %w", err)
	}
	return buf.String(), nil
}

// Template helper functions

func formatAmount(amount float64) string {
	return fmt.Sprintf("$%.2f", amount)
}

func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
