package llm

import (
	"context"
	"os"
	"os/exec"
	"testing"
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
			name:    "valid response",
			content: `{"category": "Food & Dining", "confidence": 0.85, "isNew": false}`,
			want: ClassificationResponse{
				Category:   "Food & Dining",
				Confidence: 0.85,
			},
			wantErr: false,
		},
		{
			name:    "response with extra whitespace",
			content: ` {"category": "Transportation", "confidence": 0.92, "isNew": false} `,
			want: ClassificationResponse{
				Category:   "Transportation",
				Confidence: 0.92,
			},
			wantErr: false,
		},
		{
			name:    "missing confidence defaults to 0.7",
			content: `{"category": "Groceries", "confidence": 0.7, "isNew": false}`,
			want: ClassificationResponse{
				Category:   "Groceries",
				Confidence: 0.7,
			},
			wantErr: false,
		},
		{
			name:    "missing category",
			content: `{"confidence": 0.85}`,
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
			name:    "invalid confidence format",
			content: `{"category": "Shopping", "confidence": "high"}`,
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
