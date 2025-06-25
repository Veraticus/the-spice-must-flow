package llm

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClaudeCodeClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config with defaults",
			cfg: Config{
				Provider: "claudecode",
			},
			wantErr: false,
		},
		{
			name: "custom model",
			cfg: Config{
				Provider: "claudecode",
				Model:    "opus",
			},
			wantErr: false,
		},
		{
			name: "custom temperature and tokens",
			cfg: Config{
				Provider:    "claudecode",
				Temperature: 0.5,
				MaxTokens:   200,
			},
			wantErr: false,
		},
	}

	// Skip tests if claude CLI is not available
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not available, skipping tests")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := newClaudeCodeClient(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if client == nil {
				t.Errorf("expected client but got nil")
			}
		})
	}
}

func TestClaudeCodeParseClassification(t *testing.T) {
	client := &claudeCodeClient{}

	tests := []struct {
		name    string
		content string
		want    ClassificationResponse
		wantErr bool
	}{
		{
			name: "valid response",
			content: `CATEGORY: Food & Dining
CONFIDENCE: 0.85`,
			want: ClassificationResponse{
				Category:   "Food & Dining",
				Confidence: 0.85,
			},
			wantErr: false,
		},
		{
			name: "response with extra whitespace",
			content: `
CATEGORY:  Transportation  
CONFIDENCE:  0.92  
`,
			want: ClassificationResponse{
				Category:   "Transportation",
				Confidence: 0.92,
			},
			wantErr: false,
		},
		{
			name:    "missing confidence defaults to 0.7",
			content: `CATEGORY: Groceries`,
			want: ClassificationResponse{
				Category:   "Groceries",
				Confidence: 0.7,
			},
			wantErr: false,
		},
		{
			name:    "missing category",
			content: `CONFIDENCE: 0.85`,
			want:    ClassificationResponse{},
			wantErr: true,
		},
		{
			name:    "empty response",
			content: "",
			want:    ClassificationResponse{},
			wantErr: true,
		},
		{
			name: "invalid confidence format",
			content: `CATEGORY: Shopping
CONFIDENCE: high`,
			want:    ClassificationResponse{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.parseClassification(tt.content)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none, got: %+v", got)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got.Category != tt.want.Category {
				t.Errorf("category mismatch: got %q, want %q", got.Category, tt.want.Category)
			}
			if got.Confidence != tt.want.Confidence {
				t.Errorf("confidence mismatch: got %f, want %f", got.Confidence, tt.want.Confidence)
			}
		})
	}
}

func TestClaudeCodeClassify_Integration(t *testing.T) {
	// Skip if not in integration test mode
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("skipping integration test")
	}

	// Skip if claude CLI is not available
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude CLI not available, skipping integration test")
	}

	client, err := newClaudeCodeClient(Config{
		Provider: "claudecode",
		Model:    "sonnet",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	prompt := `Classify the following transaction into one of these categories: Coffee & Dining, Food & Dining, Groceries, Transportation, Entertainment, Shopping, Office Supplies, Computer & Electronics, Healthcare, Insurance, Utilities, Home & Garden, Personal Care, Education, Travel, Gifts & Donations, Taxes, Investments, Other Expenses, Miscellaneous.

Transaction details:
- Merchant: STARBUCKS
- Amount: $5.75
- Date: 2024-01-15
- Description: STARBUCKS STORE #1234

Respond with:
CATEGORY: <category>
CONFIDENCE: <score between 0 and 1>`

	resp, err := client.Classify(ctx, prompt)
	if err != nil {
		t.Fatalf("classification failed: %v", err)
	}

	if resp.Category == "" {
		t.Errorf("expected category but got empty")
	}
	if resp.Confidence < 0 || resp.Confidence > 1 {
		t.Errorf("confidence out of range: %f", resp.Confidence)
	}

	// Log the response for debugging
	t.Logf("Classification result: Category=%s, Confidence=%f", resp.Category, resp.Confidence)
}

func TestCleanMarkdownWrapper(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "json code block with newlines",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: "{\"key\": \"value\"}",
		},
		{
			name:     "json code block without newlines",
			input:    "```json{\"key\": \"value\"}```",
			expected: "{\"key\": \"value\"}",
		},
		{
			name:     "generic code block with newlines",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: "{\"key\": \"value\"}",
		},
		{
			name:     "code block with language identifier",
			input:    "```javascript\nconst x = 1;\n```",
			expected: "const x = 1;",
		},
		{
			name:     "no code block",
			input:    "{\"key\": \"value\"}",
			expected: "{\"key\": \"value\"}",
		},
		{
			name:     "whitespace handling",
			input:    "  ```json\n  {\"key\": \"value\"}  \n```  ",
			expected: "{\"key\": \"value\"}",
		},
		{
			name:     "incomplete code block (no closing)",
			input:    "```json\n{\"key\": \"value\"}",
			expected: "```json\n{\"key\": \"value\"}",
		},
		{
			name:     "incomplete code block (no opening)",
			input:    "{\"key\": \"value\"}\n```",
			expected: "{\"key\": \"value\"}\n```",
		},
		{
			name:     "multiple lines in code block",
			input:    "```json\n{\n  \"key\": \"value\",\n  \"another\": \"test\"\n}\n```",
			expected: "{\n  \"key\": \"value\",\n  \"another\": \"test\"\n}",
		},
		{
			name:     "real Claude Code response",
			input:    "```json\n{\n  \"description\": \"Expenses related to visits, memberships, donations, or purchases at the Santa Barbara Museum of Natural History, including admission fees, gift shop items, and educational programs.\",\n  \"confidence\": 0.95\n}\n```",
			expected: "{\n  \"description\": \"Expenses related to visits, memberships, donations, or purchases at the Santa Barbara Museum of Natural History, including admission fees, gift shop items, and educational programs.\",\n  \"confidence\": 0.95\n}",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "just backticks",
			input:    "``````",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanMarkdownWrapper(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}

	// Test JSON parsing after cleaning
	t.Run("json parsing after cleaning", func(t *testing.T) {
		input := "```json\n{\"description\": \"Test category\", \"confidence\": 0.85}\n```"
		cleaned := cleanMarkdownWrapper(input)

		var result struct {
			Description string  `json:"description"`
			Confidence  float64 `json:"confidence"`
		}

		err := json.Unmarshal([]byte(cleaned), &result)
		assert.NoError(t, err)
		assert.Equal(t, "Test category", result.Description)
		assert.Equal(t, 0.85, result.Confidence)
	})
}
