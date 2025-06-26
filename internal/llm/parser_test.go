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
			want:  "```json\n{\"key\": \"value\"}",
		},
		{
			name:  "multiple code blocks (only cleans outer)",
			input: "```json\n{\"code\": \"```inner```\"}\n```",
			want:  "{\"code\": \"```inner```\"}",
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
