package sheets

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		errMsg  string
		config  Config
		wantErr bool
	}{
		{
			name: "partial oauth credentials",
			config: Config{
				ClientID:      "test-client",
				ClientSecret:  "", // Missing secret
				RefreshToken:  "test-token",
				BatchSize:     100,
				RetryAttempts: 3,
				RetryDelay:    time.Second,
			},
			wantErr: true,
			errMsg:  "no authentication method configured",
		},
		{
			name: "zero retry delay is valid",
			config: Config{
				ServiceAccountPath: "/path/to/key.json",
				BatchSize:          100,
				RetryAttempts:      0, // No retries
				RetryDelay:         0, // No delay
			},
			wantErr: false,
		},
		{
			name: "negative retry delay",
			config: Config{
				ServiceAccountPath: "/path/to/key.json",
				BatchSize:          100,
				RetryAttempts:      3,
				RetryDelay:         -1 * time.Second,
			},
			wantErr: true,
			errMsg:  "retry delay cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
