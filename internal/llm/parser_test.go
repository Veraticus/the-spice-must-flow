package llm

import (
	"testing"
)

func TestParseLLMRankings(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []CategoryRanking
		wantErr bool
	}{
		{
			name: "valid rankings",
			input: `RANKINGS:
Food & Dining|0.85
Shopping|0.45
Entertainment|0.15`,
			want: []CategoryRanking{
				{Category: "Food & Dining", Score: 0.85, IsNew: false},
				{Category: "Shopping", Score: 0.45, IsNew: false},
				{Category: "Entertainment", Score: 0.15, IsNew: false},
			},
			wantErr: false,
		},
		{
			name: "rankings with new category",
			input: `RANKINGS:
Food & Dining|0.45
Shopping|0.30

NEW_CATEGORY:
name: Coffee Shops
score: 0.75
description: Specialty coffee and tea establishments`,
			want: []CategoryRanking{
				{Category: "Food & Dining", Score: 0.45, IsNew: false},
				{Category: "Shopping", Score: 0.30, IsNew: false},
				{Category: "Coffee Shops", Score: 0.75, IsNew: true, Description: "Specialty coffee and tea establishments"},
			},
			wantErr: false,
		},
		{
			name: "rankings with spaces",
			input: `RANKINGS:
Food & Dining | 0.85
Shopping      | 0.45  
Entertainment | 0.15`,
			want: []CategoryRanking{
				{Category: "Food & Dining", Score: 0.85, IsNew: false},
				{Category: "Shopping", Score: 0.45, IsNew: false},
				{Category: "Entertainment", Score: 0.15, IsNew: false},
			},
			wantErr: false,
		},
		{
			name: "malformed lines mixed with valid",
			input: `RANKINGS:
Food & Dining|0.85
This is not a valid line
Shopping|0.45
Another bad line without pipe
Entertainment|0.15`,
			want: []CategoryRanking{
				{Category: "Food & Dining", Score: 0.85, IsNew: false},
				{Category: "Shopping", Score: 0.45, IsNew: false},
				{Category: "Entertainment", Score: 0.15, IsNew: false},
			},
			wantErr: false,
		},
		{
			name: "scores with percentage signs",
			input: `RANKINGS:
Food & Dining|85%
Shopping|45%
Entertainment|15%`,
			want: []CategoryRanking{
				{Category: "Food & Dining", Score: 0.85, IsNew: false},
				{Category: "Shopping", Score: 0.45, IsNew: false},
				{Category: "Entertainment", Score: 0.15, IsNew: false},
			},
			wantErr: false,
		},
		{
			name: "scores out of range",
			input: `RANKINGS:
Food & Dining|1.5
Shopping|-0.2
Entertainment|0.5`,
			want: []CategoryRanking{
				{Category: "Food & Dining", Score: 1.0, IsNew: false}, // Capped at 1.0
				{Category: "Shopping", Score: 0.0, IsNew: false},      // Raised to 0.0
				{Category: "Entertainment", Score: 0.5, IsNew: false},
			},
			wantErr: false,
		},
		{
			name:    "empty input",
			input:   "",
			want:    nil,
			wantErr: true,
		},
		{
			name: "no valid rankings",
			input: `RANKINGS:
This is all invalid
No pipes here
Just random text`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "new category without all fields",
			input: `RANKINGS:
Food & Dining|0.85

NEW_CATEGORY:
name: Coffee Shops
score: 0.75`,
			want: []CategoryRanking{
				{Category: "Food & Dining", Score: 0.85, IsNew: false},
				// New category not added due to missing description
			},
			wantErr: false,
		},
		{
			name: "extra text around sections",
			input: `Here are the results:

RANKINGS:
Food & Dining|0.85
Shopping|0.45

Additional notes here

NEW_CATEGORY:
name: Pet Supplies
score: 0.70
description: Items for pet care

Thank you!`,
			want: []CategoryRanking{
				{Category: "Food & Dining", Score: 0.85, IsNew: false},
				{Category: "Shopping", Score: 0.45, IsNew: false},
				{Category: "Pet Supplies", Score: 0.70, IsNew: true, Description: "Items for pet care"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLLMRankings(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLLMRankings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return // Skip further checks if we expected an error
			}

			if len(got) != len(tt.want) {
				t.Errorf("parseLLMRankings() got %d rankings, want %d", len(got), len(tt.want))
				return
			}

			for i, ranking := range got {
				if i >= len(tt.want) {
					break
				}
				want := tt.want[i]
				if ranking.Category != want.Category ||
					ranking.Score != want.Score ||
					ranking.IsNew != want.IsNew ||
					ranking.Description != want.Description {
					t.Errorf("parseLLMRankings() ranking[%d] = %+v, want %+v", i, ranking, want)
				}
			}
		})
	}
}

func TestParseClassificationWithRankings(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []CategoryRanking
		wantErr bool
	}{
		{
			name: "new rankings format",
			input: `RANKINGS:
Food & Dining|0.85
Shopping|0.45`,
			want: []CategoryRanking{
				{Category: "Food & Dining", Score: 0.85, IsNew: false},
				{Category: "Shopping", Score: 0.45, IsNew: false},
			},
			wantErr: false,
		},
		{
			name: "old format - existing category",
			input: `CATEGORY: Food & Dining
CONFIDENCE: 0.85`,
			want: []CategoryRanking{
				{Category: "Food & Dining", Score: 0.85, IsNew: false},
			},
			wantErr: false,
		},
		{
			name: "old format - new category",
			input: `CATEGORY: Pet Supplies
CONFIDENCE: 0.75
NEW: true
DESCRIPTION: Items for pet care and accessories`,
			want: []CategoryRanking{
				{Category: "Pet Supplies", Score: 0.75, IsNew: true, Description: "Items for pet care and accessories"},
			},
			wantErr: false,
		},
		{
			name: "invalid format",
			input: `This is not a valid response
Just some random text`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseClassificationWithRankings(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseClassificationWithRankings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("parseClassificationWithRankings() got %d rankings, want %d", len(got), len(tt.want))
				return
			}

			for i, ranking := range got {
				if i >= len(tt.want) {
					break
				}
				want := tt.want[i]
				if ranking.Category != want.Category ||
					ranking.Score != want.Score ||
					ranking.IsNew != want.IsNew ||
					ranking.Description != want.Description {
					t.Errorf("parseClassificationWithRankings() ranking[%d] = %+v, want %+v", i, ranking, want)
				}
			}
		})
	}
}
