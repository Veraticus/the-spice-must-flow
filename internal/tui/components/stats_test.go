package components

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/tui/themes"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNewStatsPanelModel(t *testing.T) {
	theme := themes.Default
	m := NewStatsPanelModel(theme)

	assert.Equal(t, theme, m.theme)
	assert.NotNil(t, m.categoryStats)
	assert.Equal(t, 0, len(m.categoryStats))
	assert.NotNil(t, m.progressBar)
	assert.False(t, m.progressBar.ShowPercentage)
	assert.NotZero(t, m.startTime)
	assert.Zero(t, m.lastActionTime)
	assert.Equal(t, 0, m.skipped)
	assert.Equal(t, 0, m.newCategories)
	assert.Equal(t, 0, m.total)
	assert.Equal(t, 0, m.userClassified)
	assert.Equal(t, 0, m.autoClassified)
	assert.Equal(t, 0, m.classified)
	assert.Equal(t, 0, m.width)
	assert.Equal(t, 0, m.height)
	assert.False(t, m.compact)
	assert.Equal(t, time.Duration(0), m.avgTime)
}

func TestStatsPanelModel_Update_ClassificationCompleteMsg(t *testing.T) {
	tests := []struct {
		name           string
		wantCategory   string
		classification model.Classification
		wantAuto       int
		wantUser       int
		wantSkipped    int
		wantClassified int
		wantCatCount   int
	}{
		{
			name: "AI classified transaction",
			classification: model.Classification{
				Transaction: model.Transaction{ID: "1"},
				Category:    "Groceries",
				Status:      model.StatusClassifiedByAI,
			},
			wantAuto:       1,
			wantUser:       0,
			wantSkipped:    0,
			wantClassified: 1,
			wantCategory:   "Groceries",
			wantCatCount:   1,
		},
		{
			name: "User modified transaction",
			classification: model.Classification{
				Transaction: model.Transaction{ID: "2"},
				Category:    "Shopping",
				Status:      model.StatusUserModified,
			},
			wantAuto:       0,
			wantUser:       1,
			wantSkipped:    0,
			wantClassified: 1,
			wantCategory:   "Shopping",
			wantCatCount:   1,
		},
		{
			name: "Skipped transaction",
			classification: model.Classification{
				Transaction: model.Transaction{ID: "3"},
				Category:    "",
				Status:      model.StatusUnclassified,
			},
			wantAuto:       0,
			wantUser:       0,
			wantSkipped:    1,
			wantClassified: 1,
			wantCategory:   "",
			wantCatCount:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewStatsPanelModel(themes.Default)
			msg := ClassificationCompleteMsg{Classification: tt.classification}

			updated, _ := m.Update(msg)

			assert.Equal(t, tt.wantAuto, updated.autoClassified)
			assert.Equal(t, tt.wantUser, updated.userClassified)
			assert.Equal(t, tt.wantSkipped, updated.skipped)
			assert.Equal(t, tt.wantClassified, updated.classified)
			assert.NotZero(t, updated.lastActionTime)

			if tt.wantCategory != "" {
				assert.Equal(t, tt.wantCatCount, updated.categoryStats[tt.wantCategory])
			} else {
				assert.Equal(t, 0, len(updated.categoryStats))
			}
		})
	}
}

func TestStatsPanelModel_Update_WindowSizeMsg(t *testing.T) {
	m := NewStatsPanelModel(themes.Default)

	msg := tea.WindowSizeMsg{
		Width:  120,
		Height: 40,
	}

	updated, _ := m.Update(msg)

	assert.Equal(t, 120, updated.width)
	assert.Equal(t, 40, updated.height)
	assert.Equal(t, 40, updated.progressBar.Width)
}

func TestStatsPanelModel_Update_WindowSizeMsg_Large(t *testing.T) {
	m := NewStatsPanelModel(themes.Default)

	msg := tea.WindowSizeMsg{
		Width:  200,
		Height: 80,
	}

	updated, _ := m.Update(msg)

	assert.Equal(t, 200, updated.width)
	assert.Equal(t, 80, updated.height)
	assert.Equal(t, 40, updated.progressBar.Width) // min(200-4, 40) = 40 (capped at 40)
}

func TestStatsPanelModel_Update_UnknownMessage(t *testing.T) {
	m := NewStatsPanelModel(themes.Default)
	m.classified = 5

	// Send an unknown message type
	type unknownMsg struct{}
	msg := unknownMsg{}

	updated, cmd := m.Update(msg)

	// Should return unchanged model
	assert.Equal(t, m.classified, updated.classified)
	assert.Nil(t, cmd)
}

func TestStatsPanelModel_View_Compact(t *testing.T) {
	m := NewStatsPanelModel(themes.Default)
	m.compact = true
	m.total = 100
	m.classified = 25
	m.autoClassified = 20
	m.startTime = time.Now().Add(-5 * time.Minute)

	view := m.View()

	assert.Contains(t, view, "Progress: 25/100 (25%)")
	assert.Contains(t, view, "Auto: 20")
	assert.Contains(t, view, "Time saved: 5m 0s")
}

func TestStatsPanelModel_View_Full(t *testing.T) {
	m := NewStatsPanelModel(themes.Default)
	m.compact = false
	m.total = 100
	m.classified = 50
	m.autoClassified = 40
	m.userClassified = 8
	m.skipped = 2
	m.newCategories = 3
	m.categoryStats = map[string]int{
		"Groceries":      15,
		"Transportation": 10,
		"Shopping":       8,
		"Dining":         7,
		"Entertainment":  5,
		"Utilities":      3,
		"Healthcare":     2,
	}
	m.startTime = time.Now().Add(-10 * time.Minute)
	m.avgTime = 12 * time.Second
	m.width = 80
	m.height = 24

	view := m.View()

	// Check progress section
	assert.Contains(t, view, "Progress")
	assert.Contains(t, view, "50/100 transactions (50%)")

	// Check time section
	assert.Contains(t, view, "Time")
	assert.Contains(t, view, "Elapsed:")
	assert.Contains(t, view, "Saved:")
	assert.Contains(t, view, "10m 0s") // Saved time for 40 auto classifications
	assert.Contains(t, view, "Avg/txn:")
	assert.Contains(t, view, "12s")

	// Check breakdown section
	assert.Contains(t, view, "Classification Breakdown")
	assert.Contains(t, view, "Auto-classified")
	assert.Contains(t, view, "40")
	assert.Contains(t, view, "User-modified")
	assert.Contains(t, view, "8")
	assert.Contains(t, view, "Skipped")
	assert.Contains(t, view, "2")
	assert.Contains(t, view, "New categories")
	assert.Contains(t, view, "3")

	// Check top categories section
	assert.Contains(t, view, "Top Categories")
	assert.Contains(t, view, "Groceries")
	assert.Contains(t, view, "15")
	assert.Contains(t, view, "Transport") // Truncated
	assert.Contains(t, view, "10")
	// Should only show top 5
	assert.NotContains(t, view, "Healthcare")
}

func TestStatsPanelModel_RenderProgress(t *testing.T) {
	m := NewStatsPanelModel(themes.Default)
	m.total = 200
	m.classified = 150
	m.progressBar.Width = 40

	view := m.renderProgress()

	assert.Contains(t, view, "Progress")
	assert.Contains(t, view, "150/200 transactions (75%)")
}

func TestStatsPanelModel_RenderTimeSaved(t *testing.T) {
	m := NewStatsPanelModel(themes.Default)
	m.autoClassified = 10
	m.startTime = time.Now().Add(-3 * time.Minute)
	m.avgTime = 18 * time.Second

	view := m.renderTimeSaved()

	assert.Contains(t, view, "Time")
	assert.Contains(t, view, "Elapsed:")
	assert.Contains(t, view, "3m")
	assert.Contains(t, view, "Saved:")
	assert.Contains(t, view, "2m 30s") // 10 * 15 seconds
	assert.Contains(t, view, "Avg/txn:")
	assert.Contains(t, view, "18s")
}

func TestStatsPanelModel_RenderBreakdown(t *testing.T) {
	tests := []struct {
		name         string
		wantLines    []string
		notWantLines []string
		autoCount    int
		userCount    int
		skipCount    int
		newCatCount  int
	}{
		{
			name:         "all categories present",
			autoCount:    10,
			userCount:    5,
			skipCount:    2,
			newCatCount:  1,
			wantLines:    []string{"Auto-classified:", "10", "User-modified:", "5", "Skipped:", "2", "New categories:", "1"},
			notWantLines: []string{},
		},
		{
			name:         "only some categories",
			autoCount:    10,
			userCount:    0,
			skipCount:    2,
			newCatCount:  0,
			wantLines:    []string{"Auto-classified:", "10", "Skipped:", "2"},
			notWantLines: []string{"User-modified:", "New categories:"},
		},
		{
			name:         "no classifications",
			autoCount:    0,
			userCount:    0,
			skipCount:    0,
			newCatCount:  0,
			wantLines:    []string{"Classification Breakdown"},
			notWantLines: []string{"Auto-classified:", "User-modified:", "Skipped:", "New categories:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewStatsPanelModel(themes.Default)
			m.autoClassified = tt.autoCount
			m.userClassified = tt.userCount
			m.skipped = tt.skipCount
			m.newCategories = tt.newCatCount

			view := m.renderBreakdown()

			for _, want := range tt.wantLines {
				assert.Contains(t, view, want)
			}
			for _, notWant := range tt.notWantLines {
				assert.NotContains(t, view, notWant)
			}
		})
	}
}

func TestStatsPanelModel_RenderCategoryDistribution(t *testing.T) {
	tests := []struct {
		name              string
		categoryStats     map[string]int
		wantCategories    []string
		notWantCategories []string
	}{
		{
			name: "many categories",
			categoryStats: map[string]int{
				"Cat1": 20,
				"Cat2": 15,
				"Cat3": 12,
				"Cat4": 10,
				"Cat5": 8,
				"Cat6": 5,
				"Cat7": 3,
			},
			wantCategories:    []string{"Cat1", "Cat2", "Cat3", "Cat4", "Cat5"},
			notWantCategories: []string{"Cat6", "Cat7"}, // Only top 5 shown
		},
		{
			name: "few categories",
			categoryStats: map[string]int{
				"Cat1": 10,
				"Cat2": 5,
			},
			wantCategories:    []string{"Cat1", "Cat2"},
			notWantCategories: []string{},
		},
		{
			name:              "no categories",
			categoryStats:     map[string]int{},
			wantCategories:    []string{},
			notWantCategories: []string{"Top Categories"},
		},
		{
			name: "categories with long names",
			categoryStats: map[string]int{
				"Very Long Category Name That Should Be Truncated": 10,
			},
			wantCategories:    []string{"Very Long", "..."},
			notWantCategories: []string{"Should Be Truncated"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewStatsPanelModel(themes.Default)
			m.categoryStats = tt.categoryStats

			view := m.renderCategoryDistribution()

			if len(tt.categoryStats) == 0 {
				assert.Equal(t, "", view)
			} else {
				for _, want := range tt.wantCategories {
					assert.Contains(t, view, want)
				}
				for _, notWant := range tt.notWantCategories {
					assert.NotContains(t, view, notWant)
				}
			}
		})
	}
}

func TestStatsPanelModel_UpdateStats(t *testing.T) {
	tests := []struct {
		name              string
		classifications   []model.Classification
		initialClassified int
		wantClassified    int
		wantAvgTime       bool
	}{
		{
			name:              "single classification",
			initialClassified: 0,
			classifications: []model.Classification{
				{
					Transaction: model.Transaction{ID: "1"},
					Category:    "Test",
					Status:      model.StatusClassifiedByAI,
				},
			},
			wantClassified: 1,
			wantAvgTime:    false, // No average time for first classification
		},
		{
			name:              "multiple classifications",
			initialClassified: 1,
			classifications: []model.Classification{
				{
					Transaction: model.Transaction{ID: "2"},
					Category:    "Test",
					Status:      model.StatusClassifiedByAI,
				},
			},
			wantClassified: 2,
			wantAvgTime:    true, // Should calculate average time
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewStatsPanelModel(themes.Default)
			m.classified = tt.initialClassified
			m.startTime = time.Now().Add(-10 * time.Second)

			for _, classification := range tt.classifications {
				m.updateStats(classification)
			}

			assert.Equal(t, tt.wantClassified, m.classified)
			assert.NotZero(t, m.lastActionTime)

			if tt.wantAvgTime {
				assert.NotZero(t, m.avgTime)
			} else {
				assert.Zero(t, m.avgTime)
			}
		})
	}
}

func TestStatsPanelModel_CalculateProgress(t *testing.T) {
	tests := []struct {
		name       string
		classified int
		total      int
		want       float64
	}{
		{
			name:       "zero total",
			classified: 0,
			total:      0,
			want:       0,
		},
		{
			name:       "half progress",
			classified: 50,
			total:      100,
			want:       0.5,
		},
		{
			name:       "complete",
			classified: 100,
			total:      100,
			want:       1.0,
		},
		{
			name:       "partial progress",
			classified: 33,
			total:      100,
			want:       0.33,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewStatsPanelModel(themes.Default)
			m.classified = tt.classified
			m.total = tt.total

			got := m.calculateProgress()
			assert.InDelta(t, tt.want, got, 0.001)
		})
	}
}

func TestStatsPanelModel_CalculateTimeSaved(t *testing.T) {
	tests := []struct {
		name           string
		want           string
		autoClassified int
	}{
		{
			name:           "no auto classifications",
			autoClassified: 0,
			want:           "0s",
		},
		{
			name:           "few classifications",
			autoClassified: 3,
			want:           "45s",
		},
		{
			name:           "many classifications",
			autoClassified: 10,
			want:           "2m 30s",
		},
		{
			name:           "hours of savings",
			autoClassified: 250,
			want:           "1h 2m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewStatsPanelModel(themes.Default)
			m.autoClassified = tt.autoClassified

			got := m.calculateTimeSaved()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStatsPanelModel_CalculateAvgTime(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		avgTime time.Duration
	}{
		{
			name:    "no average time",
			avgTime: 0,
			want:    "N/A",
		},
		{
			name:    "seconds only",
			avgTime: 15 * time.Second,
			want:    "15s",
		},
		{
			name:    "minutes and seconds",
			avgTime: 90 * time.Second,
			want:    "1m 30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewStatsPanelModel(themes.Default)
			m.avgTime = tt.avgTime

			got := m.calculateAvgTime()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStatsPanelModel_SetTotal(t *testing.T) {
	m := NewStatsPanelModel(themes.Default)

	m.SetTotal(150)
	assert.Equal(t, 150, m.total)
}

func TestStatsPanelModel_SetCompact(t *testing.T) {
	m := NewStatsPanelModel(themes.Default)

	m.SetCompact(true)
	assert.True(t, m.compact)

	m.SetCompact(false)
	assert.False(t, m.compact)
}

func TestStatsPanelModel_Resize(t *testing.T) {
	m := NewStatsPanelModel(themes.Default)

	m.Resize(100, 50)
	assert.Equal(t, 100, m.width)
	assert.Equal(t, 50, m.height)
	assert.Equal(t, 40, m.progressBar.Width)

	m.Resize(30, 20)
	assert.Equal(t, 30, m.width)
	assert.Equal(t, 20, m.height)
	assert.Equal(t, 26, m.progressBar.Width)
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		want     string
		duration time.Duration
	}{
		{
			name:     "seconds only",
			duration: 45 * time.Second,
			want:     "45s",
		},
		{
			name:     "exactly one minute",
			duration: 60 * time.Second,
			want:     "1m 0s",
		},
		{
			name:     "minutes and seconds",
			duration: 135 * time.Second,
			want:     "2m 15s",
		},
		{
			name:     "exactly one hour",
			duration: 3600 * time.Second,
			want:     "1h 0m",
		},
		{
			name:     "hours and minutes",
			duration: 7890 * time.Second,
			want:     "2h 11m",
		},
		{
			name:     "zero duration",
			duration: 0,
			want:     "0s",
		},
		{
			name:     "less than a second",
			duration: 500 * time.Millisecond,
			want:     "0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStatsPanelModel_CompleteFlows(t *testing.T) {
	t.Run("complete classification flow", func(t *testing.T) {
		m := NewStatsPanelModel(themes.Default)
		m.SetTotal(3)
		m.Resize(80, 24)

		// Classify first transaction (AI)
		msg1 := ClassificationCompleteMsg{
			Classification: model.Classification{
				Transaction: model.Transaction{ID: "1"},
				Category:    "Groceries",
				Status:      model.StatusClassifiedByAI,
			},
		}
		m, _ = m.Update(msg1)

		// Classify second transaction (User)
		msg2 := ClassificationCompleteMsg{
			Classification: model.Classification{
				Transaction: model.Transaction{ID: "2"},
				Category:    "Shopping",
				Status:      model.StatusUserModified,
			},
		}
		m, _ = m.Update(msg2)

		// Skip third transaction
		msg3 := ClassificationCompleteMsg{
			Classification: model.Classification{
				Transaction: model.Transaction{ID: "3"},
				Category:    "",
				Status:      model.StatusUnclassified,
			},
		}
		m, _ = m.Update(msg3)

		// Verify final state
		assert.Equal(t, 3, m.classified)
		assert.Equal(t, 1, m.autoClassified)
		assert.Equal(t, 1, m.userClassified)
		assert.Equal(t, 1, m.skipped)
		assert.Equal(t, 2, len(m.categoryStats))
		assert.Equal(t, 1, m.categoryStats["Groceries"])
		assert.Equal(t, 1, m.categoryStats["Shopping"])
		assert.Equal(t, 1.0, m.calculateProgress())
	})

	t.Run("view transitions", func(t *testing.T) {
		m := NewStatsPanelModel(themes.Default)
		m.SetTotal(100)

		// Start in full view
		view := m.View()
		assert.Contains(t, view, "Progress")
		assert.Contains(t, view, "Time")
		assert.Contains(t, view, "Classification Breakdown")

		// Switch to compact
		m.SetCompact(true)
		view = m.View()
		assert.Contains(t, view, "Progress:")
		assert.NotContains(t, view, "Classification Breakdown")
	})
}

func TestStatsPanelModel_EdgeCases(t *testing.T) {
	t.Run("empty category in stats", func(t *testing.T) {
		m := NewStatsPanelModel(themes.Default)

		// Classification with empty category
		msg := ClassificationCompleteMsg{
			Classification: model.Classification{
				Transaction: model.Transaction{ID: "1"},
				Category:    "", // Empty category
				Status:      model.StatusUserModified,
			},
		}
		m, _ = m.Update(msg)

		assert.Equal(t, 1, m.userClassified)
		assert.Equal(t, 0, len(m.categoryStats)) // Empty category not counted
	})

	t.Run("very small window size", func(t *testing.T) {
		m := NewStatsPanelModel(themes.Default)

		msg := tea.WindowSizeMsg{
			Width:  10,
			Height: 5,
		}
		m, _ = m.Update(msg)

		assert.Equal(t, 6, m.progressBar.Width)
	})

	t.Run("progress bar rendering", func(t *testing.T) {
		m := NewStatsPanelModel(themes.Default)
		m.total = 100
		m.classified = 75
		m.progressBar.Width = 20

		// The progress bar should show 75% progress
		view := m.renderProgress()
		assert.Contains(t, view, "75%")
	})

	t.Run("category distribution with equal counts", func(t *testing.T) {
		m := NewStatsPanelModel(themes.Default)
		m.categoryStats = map[string]int{
			"Cat1": 10,
			"Cat2": 10,
			"Cat3": 10,
		}

		view := m.renderCategoryDistribution()
		// All categories should be shown since they have equal counts
		assert.Contains(t, view, "Cat1")
		assert.Contains(t, view, "Cat2")
		assert.Contains(t, view, "Cat3")
	})

	t.Run("time calculations with rapid classifications", func(t *testing.T) {
		m := NewStatsPanelModel(themes.Default)
		m.startTime = time.Now()

		// Simulate rapid classifications
		for i := 0; i < 10; i++ {
			msg := ClassificationCompleteMsg{
				Classification: model.Classification{
					Transaction: model.Transaction{ID: fmt.Sprintf("%d", i)},
					Category:    "Test",
					Status:      model.StatusClassifiedByAI,
				},
			}
			m, _ = m.Update(msg)
			time.Sleep(10 * time.Millisecond) // Small delay
		}

		assert.Equal(t, 10, m.classified)
		assert.NotZero(t, m.avgTime)
		assert.Less(t, m.avgTime, time.Second) // Should be very fast
	})
}

func TestStatsPanelModel_ProgressBarScaling(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		wantWidth int
	}{
		{
			name:      "narrow window",
			width:     20,
			wantWidth: 16, // 20 - 4
		},
		{
			name:      "medium window",
			width:     60,
			wantWidth: 40, // capped at 40
		},
		{
			name:      "wide window",
			width:     200,
			wantWidth: 40, // still capped at 40
		},
		{
			name:      "very narrow",
			width:     5,
			wantWidth: 1, // 5 - 4 = 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewStatsPanelModel(themes.Default)
			msg := tea.WindowSizeMsg{Width: tt.width, Height: 24}
			m, _ = m.Update(msg)
			assert.Equal(t, tt.wantWidth, m.progressBar.Width)
		})
	}
}

func TestStatsPanelModel_CategoryStats(t *testing.T) {
	t.Run("category accumulation", func(t *testing.T) {
		m := NewStatsPanelModel(themes.Default)

		// Add multiple transactions with same category
		for i := 0; i < 5; i++ {
			msg := ClassificationCompleteMsg{
				Classification: model.Classification{
					Transaction: model.Transaction{ID: fmt.Sprintf("%d", i)},
					Category:    "Groceries",
					Status:      model.StatusClassifiedByAI,
				},
			}
			m, _ = m.Update(msg)
		}

		assert.Equal(t, 5, m.categoryStats["Groceries"])
	})

	t.Run("multiple categories", func(t *testing.T) {
		m := NewStatsPanelModel(themes.Default)
		categories := []string{"A", "B", "C", "A", "B", "A"}

		for i, cat := range categories {
			msg := ClassificationCompleteMsg{
				Classification: model.Classification{
					Transaction: model.Transaction{ID: fmt.Sprintf("%d", i)},
					Category:    cat,
					Status:      model.StatusClassifiedByAI,
				},
			}
			m, _ = m.Update(msg)
		}

		assert.Equal(t, 3, m.categoryStats["A"])
		assert.Equal(t, 2, m.categoryStats["B"])
		assert.Equal(t, 1, m.categoryStats["C"])
	})
}

func TestStatsPanelModel_RenderHelpers(t *testing.T) {
	t.Run("render with no data", func(t *testing.T) {
		m := NewStatsPanelModel(themes.Default)

		// Should not panic with empty data
		progress := m.renderProgress()
		assert.Contains(t, progress, "0/0 transactions (0%)")

		timeSaved := m.renderTimeSaved()
		assert.Contains(t, timeSaved, "N/A") // No average time

		breakdown := m.renderBreakdown()
		assert.Contains(t, breakdown, "Classification Breakdown")

		catDist := m.renderCategoryDistribution()
		assert.Equal(t, "", catDist) // Empty when no categories
	})

	t.Run("category icon integration", func(t *testing.T) {
		m := NewStatsPanelModel(themes.Default)
		m.categoryStats = map[string]int{
			"Groceries": 10,
		}

		view := m.renderCategoryDistribution()
		// Should include icon from themes.GetCategoryIcon
		assert.NotEqual(t, "", view)
		assert.Contains(t, view, "Groceries")
	})
}

func TestStatsPanelModel_FullCoverage(t *testing.T) {
	t.Run("all status types", func(t *testing.T) {
		m := NewStatsPanelModel(themes.Default)

		// Test all possible status values
		statuses := []model.ClassificationStatus{
			model.StatusClassifiedByAI,
			model.StatusUserModified,
			model.StatusUnclassified,
			model.ClassificationStatus("unknown"), // Test default case
		}

		for i, status := range statuses {
			msg := ClassificationCompleteMsg{
				Classification: model.Classification{
					Transaction: model.Transaction{ID: fmt.Sprintf("%d", i)},
					Category:    "Test",
					Status:      status,
				},
			}
			m, _ = m.Update(msg)
		}

		// Verify counts (unknown status doesn't increment specific counters)
		assert.Equal(t, 1, m.autoClassified)
		assert.Equal(t, 1, m.userClassified)
		assert.Equal(t, 1, m.skipped)
		assert.Equal(t, 4, m.classified) // All are counted as classified
	})

	t.Run("min function through progress bar", func(t *testing.T) {
		m := NewStatsPanelModel(themes.Default)

		// Test min function implicitly through progress bar width calculation
		testCases := []struct {
			width    int
			expected int
		}{
			{width: 10, expected: 6},   // 10-4 = 6 (less than 40)
			{width: 100, expected: 40}, // 100-4 = 96, but capped at 40
		}

		for _, tc := range testCases {
			m.Resize(tc.width, 24)
			assert.Equal(t, tc.expected, m.progressBar.Width)
		}
	})
}

func TestStatsPanelModel_ConcurrentSafety(t *testing.T) {
	// This test ensures the component handles rapid updates correctly
	m := NewStatsPanelModel(themes.Default)
	m.SetTotal(100)

	// Simulate rapid concurrent-like updates
	for i := 0; i < 50; i++ {
		// Alternate between different message types
		if i%2 == 0 {
			msg := ClassificationCompleteMsg{
				Classification: model.Classification{
					Transaction: model.Transaction{ID: fmt.Sprintf("%d", i)},
					Category:    fmt.Sprintf("Cat%d", i%5),
					Status:      model.StatusClassifiedByAI,
				},
			}
			m, _ = m.Update(msg)
		} else {
			msg := tea.WindowSizeMsg{
				Width:  80 + i,
				Height: 24,
			}
			m, _ = m.Update(msg)
		}
	}

	// Verify state is consistent
	assert.Equal(t, 25, m.classified) // 50 / 2 = 25 classifications
	assert.Equal(t, 129, m.width)     // Last width update: 80 + 49
	assert.NotPanics(t, func() {
		_ = m.View()
	})
}

// Helper to check string truncation.
func TestTruncateIntegration(t *testing.T) {
	m := NewStatsPanelModel(themes.Default)

	// Add category with very long name
	longName := strings.Repeat("VeryLongCategoryName", 10)
	m.categoryStats = map[string]int{
		longName: 10,
		"Short":  5,
	}

	view := m.renderCategoryDistribution()

	// Should be truncated to 12 chars
	assert.Contains(t, view, "...")
	assert.NotContains(t, view, "VeryLongCategoryNameVeryLong")
	assert.Contains(t, view, "Short")
}
