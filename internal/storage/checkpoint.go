package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CheckpointManager handles database checkpoint operations.
type CheckpointManager struct {
	db             *sql.DB
	dbPath         string
	checkpointsDir string
}

// CheckpointMetadata contains metadata about a checkpoint.
type CheckpointMetadata struct {
	CreatedAt        time.Time      `json:"created_at"`
	RowCounts        map[string]int `json:"row_counts"`
	ParentCheckpoint *string        `json:"parent_checkpoint,omitempty"`
	ID               string         `json:"id"`
	Description      string         `json:"description"`
	FileSize         int64          `json:"file_size"`
	SchemaVersion    int            `json:"schema_version"`
	IsAuto           bool           `json:"is_auto"`
}

// CheckpointInfo represents information about a checkpoint for listing.
type CheckpointInfo struct {
	ID            string
	CreatedAt     time.Time
	Description   string
	FileSize      int64
	Transactions  int
	Categories    int
	Vendors       int
	SchemaVersion int
	IsAuto        bool
}

// Common errors.
var (
	ErrCheckpointNotFound  = errors.New("checkpoint not found")
	ErrCheckpointCorrupted = errors.New("checkpoint integrity check failed")
	ErrDiskSpaceLow        = errors.New("insufficient disk space for checkpoint")
	ErrCheckpointExists    = errors.New("checkpoint already exists")
)

// NewCheckpointManager creates a new checkpoint manager.
func NewCheckpointManager(db *sql.DB, dbPath string) (*CheckpointManager, error) {
	// Determine checkpoints directory
	dir := filepath.Dir(dbPath)
	checkpointsDir := filepath.Join(dir, "checkpoints")

	// Ensure checkpoints directory exists
	if err := os.MkdirAll(checkpointsDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create checkpoints directory: %w", err)
	}

	return &CheckpointManager{
		db:             db,
		dbPath:         dbPath,
		checkpointsDir: checkpointsDir,
	}, nil
}

// Create creates a new checkpoint with the given tag and description.
func (cm *CheckpointManager) Create(ctx context.Context, tag, description string) (*CheckpointInfo, error) {
	// Generate checkpoint ID if not provided
	if tag == "" {
		tag = fmt.Sprintf("checkpoint-%s", time.Now().Format("2006-01-02-1504"))
	}

	// Validate tag (no path traversal)
	if strings.Contains(tag, "/") || strings.Contains(tag, "\\") || strings.Contains(tag, "..") {
		return nil, errors.New("invalid checkpoint tag: cannot contain path separators")
	}

	// Check if checkpoint already exists
	checkpointPath := filepath.Join(cm.checkpointsDir, tag+".db")
	if _, err := os.Stat(checkpointPath); err == nil {
		return nil, ErrCheckpointExists
	}

	// Get current schema version
	var schemaVersion int
	if err := cm.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&schemaVersion); err != nil {
		return nil, fmt.Errorf("failed to get schema version: %w", err)
	}

	// Collect row counts
	rowCounts, err := cm.collectRowCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to collect row counts: %w", err)
	}

	// Check disk space (rough estimate: current DB size * 1.1)
	dbInfo, err := os.Stat(cm.dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat database: %w", err)
	}
	requiredSpace := int64(float64(dbInfo.Size()) * 1.1)
	if !cm.hasEnoughDiskSpace(requiredSpace) {
		return nil, ErrDiskSpaceLow
	}

	// Perform SQLite backup
	if backupErr := cm.backupDatabase(ctx, checkpointPath); backupErr != nil {
		return nil, fmt.Errorf("failed to backup database: %w", backupErr)
	}

	// Get checkpoint file size
	checkpointInfo, err := os.Stat(checkpointPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat checkpoint: %w", err)
	}

	// Create metadata
	metadata := CheckpointMetadata{
		ID:            tag,
		CreatedAt:     time.Now(),
		Description:   description,
		FileSize:      checkpointInfo.Size(),
		RowCounts:     rowCounts,
		SchemaVersion: schemaVersion,
		IsAuto:        false,
	}

	// Save metadata
	metadataPath := filepath.Join(cm.checkpointsDir, tag+".meta.json")
	if err := cm.saveMetadata(metadataPath, metadata); err != nil {
		// Clean up checkpoint file on metadata save failure
		if rmErr := os.Remove(checkpointPath); rmErr != nil {
			slog.Error("failed to remove checkpoint file after metadata save failure", "error", rmErr)
		}
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	// Store metadata in database
	if err := cm.storeMetadataInDB(ctx, metadata); err != nil {
		// Non-fatal: checkpoint is still valid even if DB metadata fails
		slog.Warn("failed to store checkpoint metadata in database", "error", err)
	}

	return &CheckpointInfo{
		ID:            metadata.ID,
		CreatedAt:     metadata.CreatedAt,
		Description:   metadata.Description,
		FileSize:      metadata.FileSize,
		Transactions:  rowCounts["transactions"],
		Categories:    rowCounts["categories"],
		Vendors:       rowCounts["vendors"],
		SchemaVersion: metadata.SchemaVersion,
		IsAuto:        metadata.IsAuto,
	}, nil
}

// List returns a list of all checkpoints.
func (cm *CheckpointManager) List(_ context.Context) ([]CheckpointInfo, error) {
	entries, err := os.ReadDir(cm.checkpointsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoints directory: %w", err)
	}

	checkpoints := make([]CheckpointInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".meta.json") {
			continue
		}

		metadataPath := filepath.Join(cm.checkpointsDir, entry.Name())
		metadata, err := cm.loadMetadata(metadataPath)
		if err != nil {
			// Skip corrupted metadata files
			continue
		}

		checkpoints = append(checkpoints, CheckpointInfo{
			ID:            metadata.ID,
			CreatedAt:     metadata.CreatedAt,
			Description:   metadata.Description,
			FileSize:      metadata.FileSize,
			Transactions:  metadata.RowCounts["transactions"],
			Categories:    metadata.RowCounts["categories"],
			Vendors:       metadata.RowCounts["vendors"],
			SchemaVersion: metadata.SchemaVersion,
			IsAuto:        metadata.IsAuto,
		})
	}

	// Sort by creation time (newest first)
	for i := 0; i < len(checkpoints)-1; i++ {
		for j := i + 1; j < len(checkpoints); j++ {
			if checkpoints[i].CreatedAt.Before(checkpoints[j].CreatedAt) {
				checkpoints[i], checkpoints[j] = checkpoints[j], checkpoints[i]
			}
		}
	}

	return checkpoints, nil
}

// Restore restores the database from a checkpoint.
func (cm *CheckpointManager) Restore(_ context.Context, checkpointID string) error {
	// Validate checkpoint ID
	if strings.Contains(checkpointID, "/") || strings.Contains(checkpointID, "\\") || strings.Contains(checkpointID, "..") {
		return errors.New("invalid checkpoint ID: cannot contain path separators")
	}

	checkpointPath := filepath.Join(cm.checkpointsDir, checkpointID+".db")
	metadataPath := filepath.Join(cm.checkpointsDir, checkpointID+".meta.json")

	// Check if checkpoint exists
	if _, err := os.Stat(checkpointPath); err != nil {
		if os.IsNotExist(err) {
			return ErrCheckpointNotFound
		}
		return fmt.Errorf("failed to access checkpoint: %w", err)
	}

	// Load and verify metadata
	_, err := cm.loadMetadata(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to load checkpoint metadata: %w", err)
	}

	// Verify checkpoint integrity
	if err := cm.verifyCheckpointIntegrity(checkpointPath); err != nil {
		return ErrCheckpointCorrupted
	}

	// Close current database connection
	if err := cm.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	// Create backup of current database before restore
	backupPath := cm.dbPath + ".restore-backup"
	if err := cm.copyFile(cm.dbPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current database: %w", err)
	}

	// Restore checkpoint
	if err := cm.copyFile(checkpointPath, cm.dbPath); err != nil {
		// Attempt to restore backup on failure
		if restoreErr := cm.copyFile(backupPath, cm.dbPath); restoreErr != nil {
			slog.Error("failed to restore backup after checkpoint restore failure", "error", restoreErr)
		}
		return fmt.Errorf("failed to restore checkpoint: %w", err)
	}

	// Remove backup
	if err := os.Remove(backupPath); err != nil {
		slog.Error("failed to remove backup file", "error", err)
	}

	return nil
}

// Delete removes a checkpoint.
func (cm *CheckpointManager) Delete(ctx context.Context, checkpointID string) error {
	// Validate checkpoint ID
	if strings.Contains(checkpointID, "/") || strings.Contains(checkpointID, "\\") || strings.Contains(checkpointID, "..") {
		return errors.New("invalid checkpoint ID: cannot contain path separators")
	}

	checkpointPath := filepath.Join(cm.checkpointsDir, checkpointID+".db")
	metadataPath := filepath.Join(cm.checkpointsDir, checkpointID+".meta.json")

	// Check if checkpoint exists
	if _, err := os.Stat(checkpointPath); err != nil {
		if os.IsNotExist(err) {
			return ErrCheckpointNotFound
		}
		return fmt.Errorf("failed to access checkpoint: %w", err)
	}

	// Remove files
	if err := os.Remove(checkpointPath); err != nil {
		return fmt.Errorf("failed to remove checkpoint file: %w", err)
	}

	if err := os.Remove(metadataPath); err != nil {
		// Non-fatal: metadata might not exist
		slog.Debug("failed to remove metadata file", "error", err, "path", metadataPath)
	}

	// Remove from database
	if _, err := cm.db.ExecContext(ctx, "DELETE FROM checkpoint_metadata WHERE id = ?", checkpointID); err != nil {
		// Non-fatal: DB record might not exist
		slog.Debug("failed to remove checkpoint metadata from database", "error", err, "id", checkpointID)
	}

	return nil
}

// GetCheckpointInfo retrieves information about a specific checkpoint.
func (cm *CheckpointManager) GetCheckpointInfo(_ context.Context, checkpointID string) (*CheckpointInfo, error) {
	metadataPath := filepath.Join(cm.checkpointsDir, checkpointID+".meta.json")

	metadata, err := cm.loadMetadata(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCheckpointNotFound
		}
		return nil, fmt.Errorf("failed to load checkpoint metadata: %w", err)
	}

	return &CheckpointInfo{
		ID:            metadata.ID,
		CreatedAt:     metadata.CreatedAt,
		Description:   metadata.Description,
		FileSize:      metadata.FileSize,
		Transactions:  metadata.RowCounts["transactions"],
		Categories:    metadata.RowCounts["categories"],
		Vendors:       metadata.RowCounts["vendors"],
		SchemaVersion: metadata.SchemaVersion,
		IsAuto:        metadata.IsAuto,
	}, nil
}

// Helper methods

func (cm *CheckpointManager) collectRowCounts(ctx context.Context) (map[string]int, error) {
	counts := make(map[string]int)

	// Use explicit queries for each table to avoid SQL injection
	tableQueries := map[string]string{
		"transactions":    "SELECT COUNT(*) FROM transactions",
		"vendors":         "SELECT COUNT(*) FROM vendors",
		"categories":      "SELECT COUNT(*) FROM categories",
		"classifications": "SELECT COUNT(*) FROM classifications",
		"progress":        "SELECT COUNT(*) FROM progress",
	}

	for table, query := range tableQueries {
		var count int
		if err := cm.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			// Table might not exist in older schemas
			counts[table] = 0
			continue
		}
		counts[table] = count
	}

	return counts, nil
}

func (cm *CheckpointManager) hasEnoughDiskSpace(required int64) bool {
	// Check if we can create a file of the required size
	testFile := filepath.Join(cm.checkpointsDir, ".space-test")
	// Validate path to prevent directory traversal
	if !strings.HasPrefix(filepath.Clean(testFile), filepath.Clean(cm.checkpointsDir)) {
		return false
	}
	// #nosec G304 - testFile path is validated above
	f, err := os.Create(testFile)
	if err != nil {
		return false
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Error("failed to close test file", "error", err)
		}
		if err := os.Remove(testFile); err != nil {
			slog.Error("failed to remove test file", "error", err)
		}
	}()

	// Try to truncate to required size to check available space
	if err := f.Truncate(required); err != nil {
		return false
	}

	return true
}

func (cm *CheckpointManager) backupDatabase(ctx context.Context, destPath string) error {
	// Use SQLite's backup API for consistency
	// First, ensure WAL is checkpointed
	if _, err := cm.db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("failed to checkpoint WAL: %w", err)
	}

	// Open destination database
	destDB, err := sql.Open("sqlite3", destPath)
	if err != nil {
		return fmt.Errorf("failed to open destination database: %w", err)
	}
	defer func() {
		if err := destDB.Close(); err != nil {
			slog.Error("failed to close destination database", "error", err)
		}
	}()

	// Use VACUUM INTO for atomic copy (SQLite 3.27.0+)
	// Validate destPath to prevent SQL injection
	if strings.Contains(destPath, "'") || strings.Contains(destPath, "\"") || strings.Contains(destPath, ";") {
		return fmt.Errorf("invalid destination path: contains forbidden characters")
	}
	// Additional path validation
	if !filepath.IsAbs(destPath) || strings.Contains(destPath, "..") {
		return fmt.Errorf("invalid destination path")
	}
	// #nosec G201 - destPath is validated above to prevent SQL injection
	query := fmt.Sprintf("VACUUM INTO '%s'", destPath)
	if _, err := cm.db.ExecContext(ctx, query); err != nil {
		// Fallback to file copy if VACUUM INTO not supported
		if closeErr := destDB.Close(); closeErr != nil {
			slog.Error("failed to close destination database before fallback", "error", closeErr)
		}
		return cm.copyFile(cm.dbPath, destPath)
	}

	return nil
}

func (cm *CheckpointManager) copyFile(src, dst string) error {
	// Validate paths to prevent directory traversal
	cleanSrc := filepath.Clean(src)
	cleanDst := filepath.Clean(dst)
	if cleanSrc != src || cleanDst != dst || strings.Contains(src, "..") || strings.Contains(dst, "..") {
		return fmt.Errorf("invalid file paths")
	}

	// Create temporary file first for atomic operation
	tmpDst := dst + ".tmp"
	// Validate tmpDst path
	if !filepath.IsAbs(tmpDst) || strings.Contains(tmpDst, "..") {
		return fmt.Errorf("invalid temporary destination path")
	}

	// #nosec G304 - cleanSrc is validated above
	source, err := os.Open(cleanSrc)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := source.Close(); closeErr != nil {
			slog.Error("failed to close source file", "error", closeErr)
		}
	}()

	// #nosec G304 - tmpDst is validated above
	destination, err := os.Create(tmpDst)
	if err != nil {
		return err
	}

	if _, err := io.Copy(destination, source); err != nil {
		if closeErr := destination.Close(); closeErr != nil {
			slog.Error("failed to close destination file after copy error", "error", closeErr)
		}
		if rmErr := os.Remove(tmpDst); rmErr != nil {
			slog.Error("failed to remove temporary file after copy error", "error", rmErr)
		}
		return err
	}

	if err := destination.Close(); err != nil {
		if removeErr := os.Remove(tmpDst); removeErr != nil {
			slog.Error("failed to remove temporary file after close error", "error", removeErr)
		}
		return err
	}

	// Atomic rename
	return os.Rename(tmpDst, dst)
}

func (cm *CheckpointManager) saveMetadata(path string, metadata CheckpointMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	// Write to temporary file first
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}

	// Atomic rename
	return os.Rename(tmpPath, path)
}

func (cm *CheckpointManager) loadMetadata(path string) (*CheckpointMetadata, error) {
	// Validate path to prevent directory traversal
	if !filepath.IsAbs(path) || strings.Contains(path, "..") {
		return nil, fmt.Errorf("invalid metadata path")
	}
	// #nosec G304 - path is validated above
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var metadata CheckpointMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func (cm *CheckpointManager) verifyCheckpointIntegrity(path string) error {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("failed to close database", "error", err)
		}
	}()

	var result string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return err
	}

	if result != "ok" {
		return fmt.Errorf("integrity check failed: %s", result)
	}

	return nil
}

func (cm *CheckpointManager) storeMetadataInDB(ctx context.Context, metadata CheckpointMetadata) error {
	rowCountsJSON, err := json.Marshal(metadata.RowCounts)
	if err != nil {
		return err
	}

	query := `
		INSERT OR REPLACE INTO checkpoint_metadata 
		(id, created_at, description, file_size, row_counts, schema_version, is_auto, parent_checkpoint)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = cm.db.ExecContext(ctx, query,
		metadata.ID,
		metadata.CreatedAt,
		metadata.Description,
		metadata.FileSize,
		string(rowCountsJSON),
		metadata.SchemaVersion,
		metadata.IsAuto,
		metadata.ParentCheckpoint,
	)

	return err
}

// AutoCheckpoint creates an automatic checkpoint with a generated name.
func (cm *CheckpointManager) AutoCheckpoint(ctx context.Context, prefix string) error {
	tag := fmt.Sprintf("auto-%s-%s", prefix, time.Now().Format("2006-01-02-1504"))
	description := fmt.Sprintf("Automatic checkpoint before %s", prefix)

	// Create checkpoint - we'll need to modify the metadata after creation
	_, err := cm.Create(ctx, tag, description)
	if err != nil {
		return fmt.Errorf("failed to create auto-checkpoint: %w", err)
	}

	// Update the metadata to mark as auto
	metadataPath := filepath.Join(cm.checkpointsDir, tag+".meta.json")
	metadata, err := cm.loadMetadata(metadataPath)
	if err == nil {
		metadata.IsAuto = true
		if saveErr := cm.saveMetadata(metadataPath, *metadata); saveErr != nil {
			slog.Error("failed to save updated metadata for auto-checkpoint", "error", saveErr)
		}

		// Also update in database if possible
		if dbErr := cm.storeMetadataInDB(ctx, *metadata); dbErr != nil {
			slog.Error("failed to store metadata in database for auto-checkpoint", "error", dbErr)
		}
	}

	// Clean up old auto-checkpoints if needed
	if err := cm.cleanupOldAutoCheckpoints(ctx); err != nil {
		// Non-fatal: log but continue
		slog.Warn("failed to clean up old auto-checkpoints", "error", err)
	}

	return nil
}

func (cm *CheckpointManager) cleanupOldAutoCheckpoints(ctx context.Context) error {
	checkpoints, err := cm.List(ctx)
	if err != nil {
		return err
	}

	// Keep only the 5 most recent auto-checkpoints
	const maxAutoCheckpoints = 5
	autoCount := 0

	for _, cp := range checkpoints {
		if cp.IsAuto {
			autoCount++
			if autoCount > maxAutoCheckpoints {
				// Delete old auto-checkpoint
				if err := cm.Delete(ctx, cp.ID); err != nil {
					// Non-fatal: continue cleanup
					slog.Debug("failed to delete old auto-checkpoint during cleanup", "error", err, "checkpoint", cp.ID)
				}
			}
		}
	}

	return nil
}
