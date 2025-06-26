package llm

import (
	"testing"
)

func TestCleanMarkdownWrapper(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "json code block",
			input: "```json\n{\"key\": \"value\"}\n```",
			want:  "{\"key\": \"value\"}",
		},
		{
			name:  "generic code block",
			input: "```\n{\"key\": \"value\"}\n```",
			want:  "{\"key\": \"value\"}",
		},
		{
			name:  "code block with language",
			input: "```javascript\nconst x = 1;\n```",
			want:  "const x = 1;",
		},
		{
			name:  "no code block",
			input: "{\"key\": \"value\"}",
			want:  "{\"key\": \"value\"}",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace around code block",
			input: "  \n```json\n{\"key\": \"value\"}\n```\n  ",
			want:  "{\"key\": \"value\"}",
		},
		{
			name:  "incomplete code block",
			input: "```json\n{\"key\": \"value\"}",
			want:  "{\"key\": \"value\"}",
		},
		{
			name:  "multiple code blocks (only cleans outer)",
			input: "```json\n{\"code\": \"```inner```\"}\n```",
			want:  "{\"code\": \"```inner```\"}",
		},
		{
			name:  "text before json",
			input: "Let me categorize this transaction:\n{\"category\": \"Food\", \"confidence\": 0.95}",
			want:  "{\"category\": \"Food\", \"confidence\": 0.95}",
		},
		{
			name:  "text before and after json",
			input: "Here's my analysis:\n{\"category\": \"Food\", \"confidence\": 0.95}\nThis seems correct.",
			want:  "{\"category\": \"Food\", \"confidence\": 0.95}",
		},
		{
			name:  "nested json object",
			input: "I'll analyze this:\n{\"rankings\": [{\"category\": \"Food\", \"score\": 0.9}], \"newCategory\": {\"name\": \"Test\", \"score\": 0.5}}",
			want:  "{\"rankings\": [{\"category\": \"Food\", \"score\": 0.9}], \"newCategory\": {\"name\": \"Test\", \"score\": 0.5}}",
		},
		{
			name:  "json array with text before",
			input: "The rankings are:\n[{\"category\": \"Food\", \"score\": 0.9}, {\"category\": \"Shopping\", \"score\": 0.1}]",
			want:  "[{\"category\": \"Food\", \"score\": 0.9}, {\"category\": \"Shopping\", \"score\": 0.1}]",
		},
		{
			name:  "escaped quotes in json",
			input: "Analysis:\n{\"description\": \"This is a \\\"quoted\\\" value\", \"confidence\": 0.8}",
			want:  "{\"description\": \"This is a \\\"quoted\\\" value\", \"confidence\": 0.8}",
		},
		{
			name:  "markdown with text before json",
			input: "Let me help:\n```json\n{\"category\": \"Food\", \"confidence\": 0.95}\n```",
			want:  "{\"category\": \"Food\", \"confidence\": 0.95}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanMarkdownWrapper(tt.input)
			if got != tt.want {
				t.Errorf("cleanMarkdownWrapper() = %q, want %q", got, tt.want)
			}
		})
	}
}
