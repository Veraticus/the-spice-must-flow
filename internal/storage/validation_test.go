package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
)

func TestValidateContext(t *testing.T) {
	tests := []struct {
		ctx     context.Context
		name    string
		wantErr bool
	}{
		{
			name:    "valid context",
			ctx:     context.Background(),
			wantErr: false,
		},
		{
			name:    "nil context",
			ctx:     nil,
			wantErr: true,
		},
		{
			name: "canceled context still valid",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContext(tt.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateContext() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateString(t *testing.T) {
	tests := []struct {
		name      string
		str       string
		paramName string
		wantErr   bool
	}{
		{
			name:      "valid string",
			str:       "test",
			paramName: "param",
			wantErr:   false,
		},
		{
			name:      "empty string",
			str:       "",
			paramName: "param",
			wantErr:   true,
		},
		{
			name:      "whitespace only",
			str:       "   ",
			paramName: "param",
			wantErr:   true,
		},
		{
			name:      "string with spaces",
			str:       "  test  ",
			paramName: "param",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateString(tt.str, tt.paramName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateString() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), tt.paramName) {
				t.Errorf("validateString() error should contain param name %s, got %v", tt.paramName, err)
			}
		})
	}
}

func TestValidateTransaction(t *testing.T) {
	validDate := time.Now()
	tests := []struct {
		txn     *model.Transaction
		name    string
		errMsg  string
		wantErr bool
	}{
		{
			name: "valid transaction",
			txn: &model.Transaction{
				ID:        "txn123",
				Date:      validDate,
				Name:      "Test Transaction",
				AccountID: "acc1",
			},
			wantErr: false,
		},
		{
			name:    "nil transaction",
			txn:     nil,
			wantErr: true,
			errMsg:  "transaction",
		},
		{
			name: "missing ID",
			txn: &model.Transaction{
				Date:      validDate,
				Name:      "Test Transaction",
				AccountID: "acc1",
			},
			wantErr: true,
			errMsg:  "missing ID",
		},
		{
			name: "missing date",
			txn: &model.Transaction{
				ID:        "txn123",
				Name:      "Test Transaction",
				AccountID: "acc1",
			},
			wantErr: true,
			errMsg:  "missing date",
		},
		{
			name: "missing name",
			txn: &model.Transaction{
				ID:        "txn123",
				Date:      validDate,
				AccountID: "acc1",
			},
			wantErr: true,
			errMsg:  "missing name",
		},
		{
			name: "missing account ID",
			txn: &model.Transaction{
				ID:   "txn123",
				Date: validDate,
				Name: "Test Transaction",
			},
			wantErr: true,
			errMsg:  "missing account ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTransaction(tt.txn)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTransaction() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("validateTransaction() error should contain %s, got %v", tt.errMsg, err)
			}
		})
	}
}

func TestValidateVendor(t *testing.T) {
	tests := []struct {
		vendor  *model.Vendor
		name    string
		errMsg  string
		wantErr bool
	}{
		{
			name: "valid vendor",
			vendor: &model.Vendor{
				Name:     "Test Vendor",
				Category: "Food",
			},
			wantErr: false,
		},
		{
			name:    "nil vendor",
			vendor:  nil,
			wantErr: true,
			errMsg:  "vendor",
		},
		{
			name: "missing name",
			vendor: &model.Vendor{
				Category: "Food",
			},
			wantErr: true,
			errMsg:  "missing name",
		},
		{
			name: "missing category",
			vendor: &model.Vendor{
				Name: "Test Vendor",
			},
			wantErr: true,
			errMsg:  "missing category",
		},
		{
			name: "whitespace name",
			vendor: &model.Vendor{
				Name:     "   ",
				Category: "Food",
			},
			wantErr: true,
			errMsg:  "missing name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVendor(tt.vendor)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVendor() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("validateVendor() error should contain %s, got %v", tt.errMsg, err)
			}
		})
	}
}

func TestValidateClassification(t *testing.T) {
	validDate := time.Now()
	validTransaction := model.Transaction{
		ID:        "txn123",
		Date:      validDate,
		Name:      "Test Transaction",
		AccountID: "acc1",
	}

	tests := []struct {
		class   *model.Classification
		name    string
		errMsg  string
		wantErr bool
	}{
		{
			name: "valid classification",
			class: &model.Classification{
				Transaction: validTransaction,
				Category:    "Food",
				Status:      model.StatusUserModified,
				Confidence:  0.95,
			},
			wantErr: false,
		},
		{
			name:    "nil classification",
			class:   nil,
			wantErr: true,
			errMsg:  "classification",
		},
		{
			name: "missing category",
			class: &model.Classification{
				Transaction: validTransaction,
				Status:      model.StatusUserModified,
				Confidence:  0.95,
			},
			wantErr: true,
			errMsg:  "missing category",
		},
		{
			name: "invalid status",
			class: &model.Classification{
				Transaction: validTransaction,
				Category:    "Food",
				Status:      "INVALID_STATUS",
				Confidence:  0.95,
			},
			wantErr: true,
			errMsg:  "invalid classification status",
		},
		{
			name: "confidence too low",
			class: &model.Classification{
				Transaction: validTransaction,
				Category:    "Food",
				Status:      model.StatusUserModified,
				Confidence:  -0.1,
			},
			wantErr: true,
			errMsg:  "confidence must be between 0 and 1",
		},
		{
			name: "confidence too high",
			class: &model.Classification{
				Transaction: validTransaction,
				Category:    "Food",
				Status:      model.StatusUserModified,
				Confidence:  1.1,
			},
			wantErr: true,
			errMsg:  "confidence must be between 0 and 1",
		},
		{
			name: "confidence exactly 0",
			class: &model.Classification{
				Transaction: validTransaction,
				Category:    "Food",
				Status:      model.StatusClassifiedByAI,
				Confidence:  0,
			},
			wantErr: false,
		},
		{
			name: "confidence exactly 1",
			class: &model.Classification{
				Transaction: validTransaction,
				Category:    "Food",
				Status:      model.StatusClassifiedByRule,
				Confidence:  1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateClassification(tt.class)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateClassification() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("validateClassification() error should contain %s, got %v", tt.errMsg, err)
			}
		})
	}
}

func TestValidateTransactions(t *testing.T) {
	validDate := time.Now()
	validTxn := model.Transaction{
		ID:        "txn123",
		Date:      validDate,
		Name:      "Test Transaction",
		AccountID: "acc1",
	}

	tests := []struct {
		name    string
		errMsg  string
		txns    []model.Transaction
		wantErr bool
	}{
		{
			name:    "valid transactions",
			txns:    []model.Transaction{validTxn},
			wantErr: false,
		},
		{
			name:    "nil slice",
			txns:    nil,
			wantErr: true,
			errMsg:  "transactions",
		},
		{
			name:    "empty slice",
			txns:    []model.Transaction{},
			wantErr: true,
			errMsg:  "empty",
		},
		{
			name: "invalid transaction in slice",
			txns: []model.Transaction{
				validTxn,
				{
					Date:      validDate,
					Name:      "Missing ID",
					AccountID: "acc1",
				},
			},
			wantErr: true,
			errMsg:  "transaction at index 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTransactions(tt.txns)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTransactions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("validateTransactions() error should contain %s, got %v", tt.errMsg, err)
			}
		})
	}
}

func TestValidateProgress(t *testing.T) {
	tests := []struct {
		progress *model.ClassificationProgress
		name     string
		wantErr  bool
	}{
		{
			name: "valid progress",
			progress: &model.ClassificationProgress{
				LastProcessedID:   "txn123",
				LastProcessedDate: time.Now(),
				TotalProcessed:    10,
				StartedAt:         time.Now().Add(-1 * time.Hour),
			},
			wantErr: false,
		},
		{
			name:     "nil progress",
			progress: nil,
			wantErr:  true,
		},
		{
			name:     "empty progress is valid",
			progress: &model.ClassificationProgress{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProgress(tt.progress)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateProgress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
