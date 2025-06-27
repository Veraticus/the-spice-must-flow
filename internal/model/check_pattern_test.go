package model

import (
	"testing"
	"time"
)

func TestCheckPattern_Validate(t *testing.T) {
	tests := []struct {
		name    string
		errMsg  string
		pattern CheckPattern
		wantErr bool
	}{
		{
			name: "valid pattern with exact amount",
			pattern: CheckPattern{
				PatternName: "Monthly rent",
				AmountMin:   floatPtr(1500),
				AmountMax:   floatPtr(1500),
				Category:    "Housing",
			},
			wantErr: false,
		},
		{
			name: "valid pattern with amount range",
			pattern: CheckPattern{
				PatternName: "Utility bills",
				AmountMin:   floatPtr(100),
				AmountMax:   floatPtr(300),
				Category:    "Utilities",
			},
			wantErr: false,
		},
		{
			name: "valid pattern with day of month range",
			pattern: CheckPattern{
				PatternName:   "First week payment",
				DayOfMonthMin: intPtr(1),
				DayOfMonthMax: intPtr(7),
				Category:      "Services",
			},
			wantErr: false,
		},
		{
			name: "missing pattern name",
			pattern: CheckPattern{
				Category: "Housing",
			},
			wantErr: true,
			errMsg:  "pattern name is required",
		},
		{
			name: "missing category",
			pattern: CheckPattern{
				PatternName: "Test pattern",
			},
			wantErr: true,
			errMsg:  "category is required",
		},
		{
			name: "invalid amount range",
			pattern: CheckPattern{
				PatternName: "Invalid range",
				AmountMin:   floatPtr(500),
				AmountMax:   floatPtr(100),
				Category:    "Test",
			},
			wantErr: true,
			errMsg:  "amount min must be less than or equal to amount max",
		},
		{
			name: "invalid day of month min",
			pattern: CheckPattern{
				PatternName:   "Invalid day",
				DayOfMonthMin: intPtr(0),
				Category:      "Test",
			},
			wantErr: true,
			errMsg:  "day of month min must be between 1 and 31",
		},
		{
			name: "invalid day of month max",
			pattern: CheckPattern{
				PatternName:   "Invalid day",
				DayOfMonthMax: intPtr(32),
				Category:      "Test",
			},
			wantErr: true,
			errMsg:  "day of month max must be between 1 and 31",
		},
		{
			name: "invalid day range",
			pattern: CheckPattern{
				PatternName:   "Invalid range",
				DayOfMonthMin: intPtr(15),
				DayOfMonthMax: intPtr(10),
				Category:      "Test",
			},
			wantErr: true,
			errMsg:  "day of month min must be less than or equal to day of month max",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pattern.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() error = nil, want error containing %q", tt.errMsg)
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() error = %v, want nil", err)
				}
			}
		})
	}
}

func TestCheckPattern_Matches(t *testing.T) {
	// Test date for consistency
	testDate := time.Date(2024, 12, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		txn     Transaction
		pattern CheckPattern
		want    bool
	}{
		{
			name: "matches exact amount",
			pattern: CheckPattern{
				AmountMin: floatPtr(100),
				AmountMax: floatPtr(100),
			},
			txn: Transaction{
				Type:   "CHECK",
				Amount: 100,
				Date:   testDate,
			},
			want: true,
		},
		{
			name: "matches amount range",
			pattern: CheckPattern{
				AmountMin: floatPtr(50),
				AmountMax: floatPtr(150),
			},
			txn: Transaction{
				Type:   "CHECK",
				Amount: 100,
				Date:   testDate,
			},
			want: true,
		},
		{
			name: "no match - amount too low",
			pattern: CheckPattern{
				AmountMin: floatPtr(200),
			},
			txn: Transaction{
				Type:   "CHECK",
				Amount: 100,
				Date:   testDate,
			},
			want: false,
		},
		{
			name: "no match - amount too high",
			pattern: CheckPattern{
				AmountMax: floatPtr(50),
			},
			txn: Transaction{
				Type:   "CHECK",
				Amount: 100,
				Date:   testDate,
			},
			want: false,
		},
		{
			name: "matches day of month",
			pattern: CheckPattern{
				DayOfMonthMin: intPtr(10),
				DayOfMonthMax: intPtr(20),
			},
			txn: Transaction{
				Type: "CHECK",
				Date: testDate, // 15th of month
			},
			want: true,
		},
		{
			name: "no match - day too early",
			pattern: CheckPattern{
				DayOfMonthMin: intPtr(20),
			},
			txn: Transaction{
				Type: "CHECK",
				Date: testDate, // 15th of month
			},
			want: false,
		},
		{
			name: "no match - day too late",
			pattern: CheckPattern{
				DayOfMonthMax: intPtr(10),
			},
			txn: Transaction{
				Type: "CHECK",
				Date: testDate, // 15th of month
			},
			want: false,
		},
		{
			name:    "no match - not a check",
			pattern: CheckPattern{},
			txn: Transaction{
				Type: "DEBIT",
			},
			want: false,
		},
		{
			name: "matches complex pattern",
			pattern: CheckPattern{
				AmountMin:     floatPtr(90),
				AmountMax:     floatPtr(110),
				DayOfMonthMin: intPtr(10),
				DayOfMonthMax: intPtr(20),
			},
			txn: Transaction{
				Type:   "CHECK",
				Amount: 100,
				Date:   testDate, // 15th of month
			},
			want: true,
		},
		{
			name: "no match - complex pattern amount out of range",
			pattern: CheckPattern{
				AmountMin:     floatPtr(90),
				AmountMax:     floatPtr(110),
				DayOfMonthMin: intPtr(10),
				DayOfMonthMax: intPtr(20),
			},
			txn: Transaction{
				Type:   "CHECK",
				Amount: 150,
				Date:   testDate, // 15th of month
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pattern.Matches(tt.txn)
			if got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Helper functions.
func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}
