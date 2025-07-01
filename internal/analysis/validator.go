package analysis

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// JSONValidator implements ReportValidator for JSON report validation.
type JSONValidator struct{}

// NewJSONValidator creates a new JSON validator instance.
func NewJSONValidator() *JSONValidator {
	return &JSONValidator{}
}

// Validate checks if the report data is valid JSON and well-formed.
func (v *JSONValidator) Validate(data []byte) (*Report, error) {
	// Try to unmarshal the JSON data
	var report Report
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&report); err != nil {
		return nil, fmt.Errorf("failed to parse JSON report: %w", err)
	}

	// Validate the report structure
	if err := report.Validate(); err != nil {
		return nil, fmt.Errorf("invalid report structure: %w", err)
	}

	// Validate nested structures
	for i, issue := range report.Issues {
		if err := validateIssue(issue); err != nil {
			return nil, fmt.Errorf("invalid issue at index %d: %w", i, err)
		}
	}

	for i, pattern := range report.SuggestedPatterns {
		if err := validateSuggestedPattern(pattern); err != nil {
			return nil, fmt.Errorf("invalid suggested pattern at index %d: %w", i, err)
		}
	}

	return &report, nil
}

// ExtractError identifies the problematic section of malformed JSON.
func (v *JSONValidator) ExtractError(data []byte, err error) (section string, line int, column int) {
	// Default values if we can't extract specific location
	section = "unknown"
	line = 0
	column = 0

	// Check if this is a JSON syntax error
	if syntaxErr, ok := err.(*json.SyntaxError); ok {
		// Calculate line and column from byte offset
		line, column = calculatePosition(data, syntaxErr.Offset)

		// Extract a section around the error
		start := syntaxErr.Offset - 50
		if start < 0 {
			start = 0
		}
		end := syntaxErr.Offset + 50
		if end > int64(len(data)) {
			end = int64(len(data))
		}

		section = string(data[start:end])
		return section, line, column
	}

	// Check if this is an UnmarshalTypeError
	if typeErr, ok := err.(*json.UnmarshalTypeError); ok {
		// Calculate line and column from byte offset
		line, column = calculatePosition(data, typeErr.Offset)

		// Extract the field that caused the error
		section = fmt.Sprintf("field '%s' (expected %s)", typeErr.Field, typeErr.Type.String())
		return section, line, column
	}

	// For validation errors, try to extract relevant part from error message
	errStr := err.Error()
	if strings.Contains(errStr, "at index") {
		// Extract the problematic array element
		section = extractArrayContext(data, errStr)
	} else if strings.Contains(errStr, "field") {
		// Extract the problematic field
		section = extractFieldContext(data, errStr)
	}

	return section, line, column
}

// calculatePosition converts a byte offset to line and column numbers.
func calculatePosition(data []byte, offset int64) (line int, column int) {
	line = 1
	column = 1

	for i := int64(0); i < offset && i < int64(len(data)); i++ {
		if data[i] == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}

	return
}

// extractArrayContext attempts to extract context for array-related errors.
func extractArrayContext(data []byte, errStr string) string {
	// Try to find array context based on error message
	// This is a simple implementation that looks for array patterns

	// Look for common array field names mentioned in error
	arrayFields := []string{"issues", "suggested_patterns", "insights", "transaction_ids", "example_txn_ids"}

	for _, field := range arrayFields {
		if strings.Contains(errStr, field) {
			// Try to find this field in the JSON
			fieldPattern := fmt.Sprintf(`"%s"`, field)
			idx := strings.Index(string(data), fieldPattern)
			if idx >= 0 {
				// Extract a reasonable section around this field
				start := idx
				end := idx + 100
				if end > len(data) {
					end = len(data)
				}
				return string(data[start:end])
			}
		}
	}

	return "array element"
}

// extractFieldContext attempts to extract context for field-related errors.
func extractFieldContext(data []byte, errStr string) string {
	// Extract field name from error message
	parts := strings.Split(errStr, "'")
	if len(parts) >= 2 {
		fieldName := parts[1]

		// Try to find this field in the JSON
		fieldPattern := fmt.Sprintf(`"%s"`, fieldName)
		idx := strings.Index(string(data), fieldPattern)
		if idx >= 0 {
			// Extract a reasonable section around this field
			start := idx
			end := idx + 50
			if end > len(data) {
				end = len(data)
			}
			return string(data[start:end])
		}
	}

	return "field"
}

// validateIssue validates an individual issue structure.
func validateIssue(issue Issue) error {
	if issue.ID == "" {
		return fmt.Errorf("issue ID is required")
	}

	if issue.Type == "" {
		return fmt.Errorf("issue type is required")
	}

	// Allow any issue type - the LLM might discover new patterns
	// The 5 predefined types are just examples, not an exhaustive list
	slog.Debug("Validating issue type",
		"issue_id", issue.ID,
		"issue_type", string(issue.Type),
		"note", "allowing any issue type for flexibility")

	// Validate severity
	validSeverities := map[IssueSeverity]bool{
		SeverityCritical: true,
		SeverityHigh:     true,
		SeverityMedium:   true,
		SeverityLow:      true,
	}
	if !validSeverities[issue.Severity] {
		return fmt.Errorf("invalid issue severity: %s", issue.Severity)
	}

	if issue.Description == "" {
		return fmt.Errorf("issue description is required")
	}

	if len(issue.TransactionIDs) == 0 {
		return fmt.Errorf("issue must affect at least one transaction")
	}

	if issue.AffectedCount <= 0 {
		return fmt.Errorf("affected count must be positive")
	}

	if issue.Confidence < 0 || issue.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}

	// Validate fix if present
	if issue.Fix != nil {
		if err := validateFix(*issue.Fix); err != nil {
			return fmt.Errorf("invalid fix: %w", err)
		}
	}

	return nil
}

// validateFix validates a fix structure.
func validateFix(fix Fix) error {
	if fix.ID == "" {
		return fmt.Errorf("fix ID is required")
	}

	if fix.IssueID == "" {
		return fmt.Errorf("fix issue ID is required")
	}

	if fix.Type == "" {
		return fmt.Errorf("fix type is required")
	}

	if fix.Description == "" {
		return fmt.Errorf("fix description is required")
	}

	if len(fix.Data) == 0 {
		return fmt.Errorf("fix data is required")
	}

	return nil
}

// validateSuggestedPattern validates a suggested pattern structure.
func validateSuggestedPattern(pattern SuggestedPattern) error {
	if pattern.ID == "" {
		return fmt.Errorf("pattern ID is required")
	}

	if pattern.Name == "" {
		return fmt.Errorf("pattern name is required")
	}

	if pattern.Description == "" {
		return fmt.Errorf("pattern description is required")
	}

	if pattern.Impact == "" {
		return fmt.Errorf("pattern impact is required")
	}

	if pattern.MatchCount <= 0 {
		return fmt.Errorf("match count must be positive")
	}

	if pattern.Confidence < 0 || pattern.Confidence > 1 {
		return fmt.Errorf("confidence must be between 0 and 1")
	}

	// TODO: Add validation for the embedded pattern rule if needed

	return nil
}
