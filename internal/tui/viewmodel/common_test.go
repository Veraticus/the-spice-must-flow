package viewmodel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppView_IsReady(t *testing.T) {
	tests := []struct {
		name string
		view AppView
		want bool
	}{
		{
			name: "ready - classifying",
			view: AppView{
				State: StateClassifying,
			},
			want: true,
		},
		{
			name: "ready - stats",
			view: AppView{
				State: StateStats,
			},
			want: true,
		},
		{
			name: "ready - waiting",
			view: AppView{
				State: StateWaiting,
			},
			want: true,
		},
		{
			name: "not ready - loading",
			view: AppView{
				State: StateLoading,
			},
			want: false,
		},
		{
			name: "not ready - error",
			view: AppView{
				State: StateError,
			},
			want: false,
		},
		{
			name: "ready with components",
			view: AppView{
				State: StateClassifying,
				Classifier: &ClassifierView{
					Mode: ModeSelectingSuggestion,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.view.IsReady()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAppView_HasError(t *testing.T) {
	tests := []struct {
		name string
		view AppView
		want bool
	}{
		{
			name: "has error",
			view: AppView{
				Error: "Connection failed",
			},
			want: true,
		},
		{
			name: "no error",
			view: AppView{
				Error: "",
			},
			want: false,
		},
		{
			name: "whitespace error",
			view: AppView{
				Error: " ",
			},
			want: true,
		},
		{
			name: "error state but no error message",
			view: AppView{
				State: StateError,
				Error: "",
			},
			want: false,
		},
		{
			name: "error message but not error state",
			view: AppView{
				State: StateClassifying,
				Error: "Warning message",
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

func TestAppView_GetActiveKeyBindings(t *testing.T) {
	tests := []struct {
		name string
		view AppView
		want int
	}{
		{
			name: "all active",
			view: AppView{
				KeyBindings: []KeyBinding{
					{Key: "q", Description: "Quit", IsActive: true},
					{Key: "h", Description: "Help", IsActive: true},
					{Key: "Enter", Description: "Select", IsActive: true},
				},
			},
			want: 3,
		},
		{
			name: "some active",
			view: AppView{
				KeyBindings: []KeyBinding{
					{Key: "q", Description: "Quit", IsActive: true},
					{Key: "h", Description: "Help", IsActive: false},
					{Key: "Enter", Description: "Select", IsActive: true},
					{Key: "Esc", Description: "Cancel", IsActive: false},
				},
			},
			want: 2,
		},
		{
			name: "none active",
			view: AppView{
				KeyBindings: []KeyBinding{
					{Key: "q", Description: "Quit", IsActive: false},
					{Key: "h", Description: "Help", IsActive: false},
				},
			},
			want: 0,
		},
		{
			name: "empty bindings",
			view: AppView{
				KeyBindings: []KeyBinding{},
			},
			want: 0,
		},
		{
			name: "nil bindings",
			view: AppView{
				KeyBindings: nil,
			},
			want: 0,
		},
		{
			name: "single active",
			view: AppView{
				KeyBindings: []KeyBinding{
					{Key: "q", Description: "Quit", IsActive: false},
					{Key: "Enter", Description: "Select", IsActive: true},
					{Key: "h", Description: "Help", IsActive: false},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.view.GetActiveKeyBindings()
			assert.Len(t, got, tt.want)
			// Verify all returned bindings are actually active
			for _, kb := range got {
				assert.True(t, kb.IsActive)
			}
		})
	}
}
