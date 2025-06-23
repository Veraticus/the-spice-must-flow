package cli

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNonBlockingReader_ReadLine(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedValue string
		expectError   bool
	}{
		{
			name:          "successful read",
			input:         "test input\n",
			expectedValue: "test input",
		},
		{
			name:          "read with extra whitespace",
			input:         "  test input  \n",
			expectedValue: "test input",
		},
		{
			name:          "empty line",
			input:         "\n",
			expectedValue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			nbr := NewNonBlockingReader(reader)

			ctx := context.Background()
			result, err := nbr.ReadLine(ctx)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}
		})
	}
}

func TestNonBlockingReader_ContextCancellation(t *testing.T) {
	// Test immediate cancellation
	t.Run("immediate cancellation", func(t *testing.T) {
		reader := strings.NewReader("")
		nbr := NewNonBlockingReader(reader)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := nbr.ReadLine(ctx)
		assert.Equal(t, ErrInputCancelled, err)
	})

	// Test cancellation during read
	t.Run("cancellation during read", func(t *testing.T) {
		// Use a pipe so we can control when data is available
		pr, pw := io.Pipe()
		defer func() { _ = pr.Close() }()
		defer func() { _ = pw.Close() }()

		nbr := NewNonBlockingReader(pr)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Try to read - should timeout and return ErrInputCancelled
		_, err := nbr.ReadLine(ctx)
		assert.Equal(t, ErrInputCancelled, err)
	})
}

func TestNonBlockingReader_MultipleReads(t *testing.T) {
	input := "line1\nline2\nline3\n"
	reader := strings.NewReader(input)
	nbr := NewNonBlockingReader(reader)

	ctx := context.Background()

	// Read multiple lines
	line1, err := nbr.ReadLine(ctx)
	require.NoError(t, err)
	assert.Equal(t, "line1", line1)

	line2, err := nbr.ReadLine(ctx)
	require.NoError(t, err)
	assert.Equal(t, "line2", line2)

	line3, err := nbr.ReadLine(ctx)
	require.NoError(t, err)
	assert.Equal(t, "line3", line3)
}
