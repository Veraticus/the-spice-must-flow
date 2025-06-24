package viewmodel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClassifierMode_String(t *testing.T) {
	tests := []struct {
		name string
		want string
		mode ClassifierMode
	}{
		{
			name: "selecting suggestion",
			mode: ModeSelectingSuggestion,
			want: "SelectingSuggestion",
		},
		{
			name: "entering custom",
			mode: ModeEnteringCustom,
			want: "EnteringCustom",
		},
		{
			name: "selecting category",
			mode: ModeSelectingCategory,
			want: "SelectingCategory",
		},
		{
			name: "confirming",
			mode: ModeConfirming,
			want: "Confirming",
		},
		{
			name: "unknown mode",
			mode: ClassifierMode(99),
			want: "Unknown(99)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mode.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAppState_String(t *testing.T) {
	tests := []struct {
		name  string
		want  string
		state AppState
	}{
		{
			name:  "loading",
			state: StateLoading,
			want:  "Loading",
		},
		{
			name:  "classifying",
			state: StateClassifying,
			want:  "Classifying",
		},
		{
			name:  "stats",
			state: StateStats,
			want:  "Stats",
		},
		{
			name:  "error",
			state: StateError,
			want:  "Error",
		},
		{
			name:  "waiting",
			state: StateWaiting,
			want:  "Waiting",
		},
		{
			name:  "unknown state",
			state: AppState(99),
			want:  "Unknown(99)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.String()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClassifierView_GetVisibleCategories(t *testing.T) {
	categories := []CategoryView{
		{Name: "Cat1"},
		{Name: "Cat2"},
		{Name: "Cat3"},
		{Name: "Cat4"},
		{Name: "Cat5"},
	}

	tests := []struct {
		name              string
		wantFirstCategory string
		view              ClassifierView
		wantLen           int
	}{
		{
			name: "show all when not limiting",
			view: ClassifierView{
				Categories:        categories,
				ShowAllCategories: false,
				MaxDisplayItems:   3,
				CategoryOffset:    0,
			},
			wantLen:           5,
			wantFirstCategory: "Cat1",
		},
		{
			name: "limit display items",
			view: ClassifierView{
				Categories:        categories,
				ShowAllCategories: true,
				MaxDisplayItems:   3,
				CategoryOffset:    0,
			},
			wantLen:           3,
			wantFirstCategory: "Cat1",
		},
		{
			name: "offset categories",
			view: ClassifierView{
				Categories:        categories,
				ShowAllCategories: true,
				MaxDisplayItems:   3,
				CategoryOffset:    2,
			},
			wantLen:           3,
			wantFirstCategory: "Cat3",
		},
		{
			name: "offset near end",
			view: ClassifierView{
				Categories:        categories,
				ShowAllCategories: true,
				MaxDisplayItems:   3,
				CategoryOffset:    3,
			},
			wantLen:           2,
			wantFirstCategory: "Cat4",
		},
		{
			name: "empty categories",
			view: ClassifierView{
				Categories:        []CategoryView{},
				ShowAllCategories: true,
				MaxDisplayItems:   3,
				CategoryOffset:    0,
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.view.GetVisibleCategories()
			assert.Len(t, got, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, tt.wantFirstCategory, got[0].Name)
			}
		})
	}
}

func TestClassifierView_CanScroll(t *testing.T) {
	tests := []struct {
		name           string
		view           ClassifierView
		wantScrollUp   bool
		wantScrollDown bool
	}{
		{
			name: "can scroll down only",
			view: ClassifierView{
				Categories:        make([]CategoryView, 10),
				ShowAllCategories: true,
				MaxDisplayItems:   5,
				CategoryOffset:    0,
			},
			wantScrollUp:   false,
			wantScrollDown: true,
		},
		{
			name: "can scroll both ways",
			view: ClassifierView{
				Categories:        make([]CategoryView, 10),
				ShowAllCategories: true,
				MaxDisplayItems:   5,
				CategoryOffset:    2,
			},
			wantScrollUp:   true,
			wantScrollDown: true,
		},
		{
			name: "can scroll up only",
			view: ClassifierView{
				Categories:        make([]CategoryView, 10),
				ShowAllCategories: true,
				MaxDisplayItems:   5,
				CategoryOffset:    5,
			},
			wantScrollUp:   true,
			wantScrollDown: false,
		},
		{
			name: "cannot scroll when showing all",
			view: ClassifierView{
				Categories:        make([]CategoryView, 10),
				ShowAllCategories: false,
				MaxDisplayItems:   5,
				CategoryOffset:    2,
			},
			wantScrollUp:   false,
			wantScrollDown: false,
		},
		{
			name: "cannot scroll when all fit",
			view: ClassifierView{
				Categories:        make([]CategoryView, 3),
				ShowAllCategories: true,
				MaxDisplayItems:   5,
				CategoryOffset:    0,
			},
			wantScrollUp:   false,
			wantScrollDown: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantScrollUp, tt.view.CanScrollUp())
			assert.Equal(t, tt.wantScrollDown, tt.view.CanScrollDown())
		})
	}
}

func TestClassifierView_GetSelectedCategory(t *testing.T) {
	tests := []struct {
		wantCategory *CategoryView
		name         string
		view         ClassifierView
	}{
		{
			name: "has selected category",
			view: ClassifierView{
				Categories: []CategoryView{
					{Name: "Cat1", IsSelected: false},
					{Name: "Cat2", IsSelected: true},
					{Name: "Cat3", IsSelected: false},
				},
			},
			wantCategory: &CategoryView{Name: "Cat2", IsSelected: true},
		},
		{
			name: "no selected category",
			view: ClassifierView{
				Categories: []CategoryView{
					{Name: "Cat1", IsSelected: false},
					{Name: "Cat2", IsSelected: false},
				},
			},
			wantCategory: nil,
		},
		{
			name: "empty categories",
			view: ClassifierView{
				Categories: []CategoryView{},
			},
			wantCategory: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.view.GetSelectedCategory()
			if tt.wantCategory == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, tt.wantCategory.Name, got.Name)
				assert.Equal(t, tt.wantCategory.IsSelected, got.IsSelected)
			}
		})
	}
}

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		name   string
		want   string
		amount float64
	}{
		{
			name:   "positive amount",
			amount: 123.45,
			want:   "$123.45",
		},
		{
			name:   "negative amount",
			amount: -67.89,
			want:   "$-67.89",
		},
		{
			name:   "zero",
			amount: 0,
			want:   "$0.00",
		},
		{
			name:   "rounds to two decimals",
			amount: 123.456,
			want:   "$123.46",
		},
		{
			name:   "large amount",
			amount: 12345678.90,
			want:   "$12345678.90",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatAmount(tt.amount)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatAmountWithSign(t *testing.T) {
	tests := []struct {
		name     string
		want     string
		amount   float64
		isCredit bool
	}{
		{
			name:     "credit amount",
			amount:   100.00,
			isCredit: true,
			want:     "+$100.00",
		},
		{
			name:     "debit amount",
			amount:   50.00,
			isCredit: false,
			want:     "-$50.00",
		},
		{
			name:     "zero credit",
			amount:   0,
			isCredit: true,
			want:     "+$0.00",
		},
		{
			name:     "zero debit",
			amount:   0,
			isCredit: false,
			want:     "-$0.00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatAmountWithSign(tt.amount, tt.isCredit)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		want   string
		maxLen int
	}{
		{
			name:   "short string",
			s:      "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length",
			s:      "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "needs truncation",
			s:      "hello world",
			maxLen: 8,
			want:   "hello...",
		},
		{
			name:   "very short max",
			s:      "hello",
			maxLen: 3,
			want:   "hel",
		},
		{
			name:   "empty string",
			s:      "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "zero max length",
			s:      "hello",
			maxLen: 0,
			want:   "",
		},
		{
			name:   "one char max",
			s:      "hello",
			maxLen: 1,
			want:   "h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateString(tt.s, tt.maxLen)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProgressView_GetProgressPercentage(t *testing.T) {
	tests := []struct {
		name     string
		progress ProgressView
		want     float64
	}{
		{
			name:     "half complete",
			progress: ProgressView{Current: 50, Total: 100},
			want:     50.0,
		},
		{
			name:     "complete",
			progress: ProgressView{Current: 100, Total: 100},
			want:     100.0,
		},
		{
			name:     "empty",
			progress: ProgressView{Current: 0, Total: 100},
			want:     0.0,
		},
		{
			name:     "zero total",
			progress: ProgressView{Current: 50, Total: 0},
			want:     0.0,
		},
		{
			name:     "over 100%",
			progress: ProgressView{Current: 110, Total: 100},
			want:     110.0,
		},
		{
			name:     "fractional",
			progress: ProgressView{Current: 1, Total: 3},
			want:     33.333333333333336,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.progress.GetProgressPercentage()
			assert.InDelta(t, tt.want, got, 0.0001)
		})
	}
}

func TestProgressView_GetProgressBar(t *testing.T) {
	tests := []struct {
		name     string
		want     string
		progress ProgressView
		width    int
	}{
		{
			name:     "half complete",
			progress: ProgressView{Current: 50, Total: 100},
			width:    10,
			want:     "█████░░░░░",
		},
		{
			name:     "complete",
			progress: ProgressView{Current: 100, Total: 100},
			width:    10,
			want:     "██████████",
		},
		{
			name:     "empty",
			progress: ProgressView{Current: 0, Total: 100},
			width:    10,
			want:     "░░░░░░░░░░",
		},
		{
			name:     "zero width",
			progress: ProgressView{Current: 50, Total: 100},
			width:    0,
			want:     "",
		},
		{
			name:     "negative width",
			progress: ProgressView{Current: 50, Total: 100},
			width:    -5,
			want:     "",
		},
		{
			name:     "small progress",
			progress: ProgressView{Current: 1, Total: 100},
			width:    10,
			want:     "░░░░░░░░░░",
		},
		{
			name:     "over 100%",
			progress: ProgressView{Current: 110, Total: 100},
			width:    10,
			want:     "██████████",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.progress.GetProgressBar(tt.width)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTransactionListView_IsAnyTransactionSelected(t *testing.T) {
	tests := []struct {
		name string
		view TransactionListView
		want bool
	}{
		{
			name: "has selected",
			view: TransactionListView{
				Transactions: []TransactionItemView{
					{IsSelected: false},
					{IsSelected: true},
					{IsSelected: false},
				},
			},
			want: true,
		},
		{
			name: "none selected",
			view: TransactionListView{
				Transactions: []TransactionItemView{
					{IsSelected: false},
					{IsSelected: false},
				},
			},
			want: false,
		},
		{
			name: "empty list",
			view: TransactionListView{
				Transactions: []TransactionItemView{},
			},
			want: false,
		},
		{
			name: "all selected",
			view: TransactionListView{
				Transactions: []TransactionItemView{
					{IsSelected: true},
					{IsSelected: true},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.view.IsAnyTransactionSelected()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTransactionListView_GetSelectedTransactions(t *testing.T) {
	tests := []struct {
		name string
		view TransactionListView
		want int
	}{
		{
			name: "some selected",
			view: TransactionListView{
				Transactions: []TransactionItemView{
					{CategoryName: "A", IsSelected: false},
					{CategoryName: "B", IsSelected: true},
					{CategoryName: "C", IsSelected: false},
					{CategoryName: "D", IsSelected: true},
				},
			},
			want: 2,
		},
		{
			name: "none selected",
			view: TransactionListView{
				Transactions: []TransactionItemView{
					{IsSelected: false},
					{IsSelected: false},
				},
			},
			want: 0,
		},
		{
			name: "empty list",
			view: TransactionListView{
				Transactions: []TransactionItemView{},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.view.GetSelectedTransactions()
			assert.Len(t, got, tt.want)
			for _, txn := range got {
				assert.True(t, txn.IsSelected)
			}
		})
	}
}

func TestGetConfidenceBar(t *testing.T) {
	tests := []struct {
		name       string
		want       string
		confidence float64
		width      int
	}{
		{
			name:       "0% confidence",
			confidence: 0.0,
			width:      10,
			want:       "░░░░░░░░░░",
		},
		{
			name:       "50% confidence",
			confidence: 0.5,
			width:      10,
			want:       "█████░░░░░",
		},
		{
			name:       "100% confidence",
			confidence: 1.0,
			width:      10,
			want:       "██████████",
		},
		{
			name:       "25% confidence",
			confidence: 0.25,
			width:      8,
			want:       "██░░░░░░",
		},
		{
			name:       "negative confidence clamped",
			confidence: -0.5,
			width:      10,
			want:       "░░░░░░░░░░",
		},
		{
			name:       "over 100% confidence clamped",
			confidence: 1.5,
			width:      10,
			want:       "██████████",
		},
		{
			name:       "zero width",
			confidence: 0.5,
			width:      0,
			want:       "",
		},
		{
			name:       "negative width",
			confidence: 0.5,
			width:      -5,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetConfidenceBar(tt.confidence, tt.width)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetConfidenceLevel(t *testing.T) {
	tests := []struct {
		name       string
		want       string
		confidence float64
	}{
		{
			name:       "high confidence",
			confidence: 0.85,
			want:       "High",
		},
		{
			name:       "exactly 80%",
			confidence: 0.8,
			want:       "High",
		},
		{
			name:       "medium confidence",
			confidence: 0.65,
			want:       "Medium",
		},
		{
			name:       "exactly 50%",
			confidence: 0.5,
			want:       "Medium",
		},
		{
			name:       "low confidence",
			confidence: 0.3,
			want:       "Low",
		},
		{
			name:       "zero confidence",
			confidence: 0,
			want:       "Low",
		},
		{
			name:       "negative confidence",
			confidence: -0.5,
			want:       "Low",
		},
		{
			name:       "over 100%",
			confidence: 1.5,
			want:       "High",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetConfidenceLevel(tt.confidence)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSanitizeForDisplay(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{
			name: "normal string",
			s:    "Hello World",
			want: "Hello World",
		},
		{
			name: "control characters",
			s:    "Hello\x00\x01\x02World",
			want: "Hello World",
		},
		{
			name: "newlines removed",
			s:    "Hello\nWorld",
			want: "Hello World",
		},
		{
			name: "tabs converted to single space",
			s:    "Hello\tWorld",
			want: "Hello World",
		},
		{
			name: "multiple spaces collapsed",
			s:    "Hello    World",
			want: "Hello World",
		},
		{
			name: "leading/trailing spaces",
			s:    "  Hello World  ",
			want: "Hello World",
		},
		{
			name: "empty string",
			s:    "",
			want: "",
		},
		{
			name: "only whitespace",
			s:    "   \n\t  ",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeForDisplay(tt.s)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatDate(t *testing.T) {
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{
			name: "normal date",
			t:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			want: "2024-01-15",
		},
		{
			name: "single digit month/day",
			t:    time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC),
			want: "2024-03-05",
		},
		{
			name: "end of year",
			t:    time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			want: "2024-12-31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDate(tt.t)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatDateShort(t *testing.T) {
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{
			name: "normal date",
			t:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			want: "Jan 15",
		},
		{
			name: "single digit day",
			t:    time.Date(2024, 3, 5, 0, 0, 0, 0, time.UTC),
			want: "Mar 05",
		},
		{
			name: "end of year",
			t:    time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			want: "Dec 31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDateShort(tt.t)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		want string
		d    time.Duration
	}{
		{
			name: "seconds only",
			d:    30 * time.Second,
			want: "30s",
		},
		{
			name: "exactly one minute",
			d:    60 * time.Second,
			want: "1m",
		},
		{
			name: "minutes and seconds",
			d:    90 * time.Second,
			want: "1m 30s",
		},
		{
			name: "exactly one hour",
			d:    60 * time.Minute,
			want: "1h",
		},
		{
			name: "hours and minutes",
			d:    75 * time.Minute,
			want: "1h 15m",
		},
		{
			name: "zero duration",
			d:    0,
			want: "0s",
		},
		{
			name: "multiple hours",
			d:    150 * time.Minute,
			want: "2h 30m",
		},
		{
			name: "just under a minute",
			d:    59 * time.Second,
			want: "59s",
		},
		{
			name: "just under an hour",
			d:    59*time.Minute + 30*time.Second,
			want: "59m 30s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.d)
			assert.Equal(t, tt.want, got)
		})
	}
}
