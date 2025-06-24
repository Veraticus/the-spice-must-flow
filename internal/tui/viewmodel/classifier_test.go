package viewmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCategoryView_IsValidSelection(t *testing.T) {
	tests := []struct {
		name     string
		category CategoryView
		want     bool
	}{
		{
			name: "valid category",
			category: CategoryView{
				ID:   1,
				Name: "Groceries",
			},
			want: true,
		},
		{
			name: "invalid - no ID",
			category: CategoryView{
				ID:   0,
				Name: "Groceries",
			},
			want: false,
		},
		{
			name: "invalid - negative ID",
			category: CategoryView{
				ID:   -1,
				Name: "Groceries",
			},
			want: false,
		},
		{
			name: "invalid - no name",
			category: CategoryView{
				ID:   1,
				Name: "",
			},
			want: false,
		},
		{
			name: "invalid - both missing",
			category: CategoryView{
				ID:   0,
				Name: "",
			},
			want: false,
		},
		{
			name: "valid with extra fields",
			category: CategoryView{
				ID:             42,
				Name:           "Transportation",
				Icon:           "🚗",
				Confidence:     0.95,
				IsAISuggestion: true,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.category.IsValidSelection()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClassifierView_HasError(t *testing.T) {
	tests := []struct {
		name string
		view ClassifierView
		want bool
	}{
		{
			name: "has error",
			view: ClassifierView{
				Error: "Something went wrong",
			},
			want: true,
		},
		{
			name: "no error",
			view: ClassifierView{
				Error: "",
			},
			want: false,
		},
		{
			name: "whitespace error",
			view: ClassifierView{
				Error: " ",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.view.HasError()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClassifierView_IsCustomMode(t *testing.T) {
	tests := []struct {
		name string
		view ClassifierView
		want bool
	}{
		{
			name: "is custom mode",
			view: ClassifierView{
				Mode: ModeEnteringCustom,
			},
			want: true,
		},
		{
			name: "selecting suggestion mode",
			view: ClassifierView{
				Mode: ModeSelectingSuggestion,
			},
			want: false,
		},
		{
			name: "selecting category mode",
			view: ClassifierView{
				Mode: ModeSelectingCategory,
			},
			want: false,
		},
		{
			name: "confirming mode",
			view: ClassifierView{
				Mode: ModeConfirming,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.view.IsCustomMode()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClassifierView_HasSuggestions(t *testing.T) {
	tests := []struct {
		name string
		view ClassifierView
		want bool
	}{
		{
			name: "has AI suggestions",
			view: ClassifierView{
				Categories: []CategoryView{
					{Name: "Cat1", IsAISuggestion: false},
					{Name: "Cat2", IsAISuggestion: true},
					{Name: "Cat3", IsAISuggestion: false},
				},
			},
			want: true,
		},
		{
			name: "all AI suggestions",
			view: ClassifierView{
				Categories: []CategoryView{
					{Name: "Cat1", IsAISuggestion: true},
					{Name: "Cat2", IsAISuggestion: true},
				},
			},
			want: true,
		},
		{
			name: "no AI suggestions",
			view: ClassifierView{
				Categories: []CategoryView{
					{Name: "Cat1", IsAISuggestion: false},
					{Name: "Cat2", IsAISuggestion: false},
				},
			},
			want: false,
		},
		{
			name: "empty categories",
			view: ClassifierView{
				Categories: []CategoryView{},
			},
			want: false,
		},
		{
			name: "nil categories",
			view: ClassifierView{
				Categories: nil,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.view.HasSuggestions()
			assert.Equal(t, tt.want, got)
		})
	}
}
