package analysis

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// TempFileManager handles creation and cleanup of temporary JSON files for analysis.
type TempFileManager struct {
	baseDir string
}

// NewTempFileManager creates a new temp file manager.
func NewTempFileManager(baseDir string) *TempFileManager {
	return &TempFileManager{
		baseDir: baseDir,
	}
}

// CreateTransactionFile writes transactions to a temporary JSON file
// Returns the file path and a cleanup function.
func (t *TempFileManager) CreateTransactionFile(data map[string]interface{}) (string, func(), error) {
	// Ensure base directory exists
	if err := os.MkdirAll(t.baseDir, 0700); err != nil {
		return "", nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Generate unique filename
	filename := fmt.Sprintf("transactions_%s.json", uuid.New().String())
	filePath := filepath.Join(t.baseDir, filename)

	// Marshal data to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	// Write file with secure permissions
	if err := os.WriteFile(filePath, jsonData, 0600); err != nil {
		return "", nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	// Return cleanup function
	cleanup := func() {
		_ = os.Remove(filePath)
	}

	return filePath, cleanup, nil
}

// GetBaseDir returns the base directory for temp files.
func (t *TempFileManager) GetBaseDir() string {
	return t.baseDir
}
