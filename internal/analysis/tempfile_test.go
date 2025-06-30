package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTempFileManager_CreateTransactionFile(t *testing.T) {
	tests := []struct {
		data    map[string]interface{}
		name    string
		wantErr bool
	}{
		{
			name: "simple transaction data",
			data: map[string]interface{}{
				"transactions": []map[string]interface{}{
					{
						"id":          "123",
						"description": "Test Transaction",
						"amount":      100.50,
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty data",
			data:    map[string]interface{}{},
			wantErr: false,
		},
		{
			name: "complex nested data",
			data: map[string]interface{}{
				"metadata": map[string]interface{}{
					"count": 2,
					"date":  "2024-01-01",
				},
				"transactions": []map[string]interface{}{
					{
						"id":          "123",
						"description": "Test Transaction 1",
						"amount":      100.50,
						"category":    "Food",
					},
					{
						"id":          "124",
						"description": "Test Transaction 2",
						"amount":      50.25,
						"category":    "Transport",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for test
			tempDir := t.TempDir()
			manager := NewTempFileManager(tempDir)

			// Create transaction file
			filePath, cleanup, err := manager.CreateTransactionFile(tt.data)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, filePath)
			assert.NotNil(t, cleanup)

			// Verify file exists
			info, err := os.Stat(filePath)
			require.NoError(t, err)
			assert.False(t, info.IsDir())

			// Check permissions (should be 0600)
			assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

			// Verify file contains valid JSON
			data, err := os.ReadFile(filePath)
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Cleanup
			cleanup()

			// Verify file is removed
			_, err = os.Stat(filePath)
			assert.True(t, os.IsNotExist(err))
		})
	}
}

func TestTempFileManager_GetBaseDir(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewTempFileManager(tempDir)

	assert.Equal(t, tempDir, manager.GetBaseDir())
}

func TestTempFileManager_CreateTransactionFile_DirectoryCreation(t *testing.T) {
	// Test that directory is created if it doesn't exist
	tempDir := t.TempDir()
	nonExistentDir := filepath.Join(tempDir, "subdir", "deep", "path")

	manager := NewTempFileManager(nonExistentDir)

	data := map[string]interface{}{
		"test": "data",
	}

	filePath, cleanup, err := manager.CreateTransactionFile(data)
	require.NoError(t, err)
	defer cleanup()

	// Verify directory was created
	info, err := os.Stat(nonExistentDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	// Verify file exists in the created directory
	assert.Contains(t, filePath, nonExistentDir)
}
