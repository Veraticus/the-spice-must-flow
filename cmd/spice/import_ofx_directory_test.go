package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandGlobsWithDirectory(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	// Create test OFX files
	testFiles := []string{
		"checking.ofx",
		"savings.OFX",
		"credit.qfx",
		"report.pdf", // Non-OFX file
		"subdir/another.ofx",
		"subdir/nested/deep.OFX",
	}

	for _, file := range testFiles {
		path := filepath.Join(tmpDir, file)
		dir := filepath.Dir(path)
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)

		err = os.WriteFile(path, []byte("test"), 0644)
		require.NoError(t, err)
	}

	// Test directory expansion
	args := []string{tmpDir}

	// Simulate the file collection logic from runImportOFX
	var allFiles []string
	for _, pattern := range args {
		info, err := os.Stat(pattern)
		if err == nil && info.IsDir() {
			err = filepath.Walk(pattern, func(path string, fileInfo os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !fileInfo.IsDir() {
					ext := filepath.Ext(path)
					// Convert to lowercase for comparison
					extLower := ext
					if len(ext) > 0 {
						extLower = "." + string([]byte{byte(ext[1] | 0x20)})
						if len(ext) > 2 {
							extLower += string([]byte{byte(ext[2] | 0x20)})
						}
						if len(ext) > 3 {
							extLower += string([]byte{byte(ext[3] | 0x20)})
						}
					}
					if extLower == ".ofx" || extLower == ".qfx" {
						allFiles = append(allFiles, path)
					}
				}
				return nil
			})
			require.NoError(t, err)
		}
	}

	// Verify we found all OFX/QFX files
	assert.Len(t, allFiles, 5) // Should find 5 OFX/QFX files, not the PDF

	// Verify specific files were found
	expectedFiles := map[string]bool{
		"checking.ofx":           false,
		"savings.OFX":            false,
		"credit.qfx":             false,
		"subdir/another.ofx":     false,
		"subdir/nested/deep.OFX": false,
	}

	for _, file := range allFiles {
		relPath, err := filepath.Rel(tmpDir, file)
		require.NoError(t, err)
		expectedFiles[relPath] = true
	}

	// All expected files should be found
	for file, found := range expectedFiles {
		assert.True(t, found, "Expected to find %s", file)
	}
}
