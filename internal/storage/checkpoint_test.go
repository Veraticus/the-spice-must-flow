package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*sql.DB, string, func()) {
	tmpDir, err := os.MkdirTemp("", "checkpoint-test-*")
	require.NoError(t, err)

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)

	// Create test tables
	queries := []string{
		`CREATE TABLE transactions (
			id TEXT PRIMARY KEY,
			hash TEXT UNIQUE NOT NULL,
			date DATETIME NOT NULL,
			name TEXT NOT NULL,
			merchant_name TEXT,
			amount REAL NOT NULL
		)`,
		`CREATE TABLE vendors (
			name TEXT PRIMARY KEY,
			category TEXT NOT NULL
		)`,
		`CREATE TABLE categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL
		)`,
		`CREATE TABLE classifications (
			transaction_id TEXT PRIMARY KEY,
			category TEXT NOT NULL
		)`,
		`CREATE TABLE progress (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			last_processed_id TEXT
		)`,
		`CREATE TABLE checkpoint_metadata (
			id TEXT PRIMARY KEY,
			created_at DATETIME NOT NULL,
			description TEXT,
			file_size INTEGER,
			row_counts TEXT,
			schema_version INTEGER,
			is_auto BOOLEAN DEFAULT 0,
			parent_checkpoint TEXT
		)`,
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		require.NoError(t, err)
	}

	// Set schema version
	_, err = db.Exec("PRAGMA user_version = 7")
	require.NoError(t, err)

	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, dbPath, cleanup
}

func insertTestData(t *testing.T, db *sql.DB) {
	// Insert test transactions
	_, err := db.Exec(`
		INSERT INTO transactions (id, hash, date, name, merchant_name, amount)
		VALUES 
		('tx1', 'hash1', '2024-01-01', 'Purchase 1', 'Store A', 10.50),
		('tx2', 'hash2', '2024-01-02', 'Purchase 2', 'Store B', 25.00),
		('tx3', 'hash3', '2024-01-03', 'Purchase 3', 'Store C', 100.00)
	`)
	require.NoError(t, err)

	// Insert test vendors
	_, err = db.Exec(`
		INSERT INTO vendors (name, category)
		VALUES 
		('Store A', 'Groceries'),
		('Store B', 'Entertainment')
	`)
	require.NoError(t, err)

	// Insert test categories
	_, err = db.Exec(`
		INSERT INTO categories (name)
		VALUES 
		('Groceries'),
		('Entertainment'),
		('Transportation')
	`)
	require.NoError(t, err)
}

func TestCheckpointManager_Create(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	insertTestData(t, db)

	manager, err := NewCheckpointManager(db, dbPath)
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		errType     error
		name        string
		tag         string
		description string
		wantErr     bool
	}{
		{
			name:        "Create checkpoint with tag",
			tag:         "test-checkpoint",
			description: "Test checkpoint",
			wantErr:     false,
		},
		{
			name:        "Create checkpoint without tag (auto-generated)",
			tag:         "",
			description: "Auto checkpoint",
			wantErr:     false,
		},
		{
			name:        "Create checkpoint with invalid tag (path traversal)",
			tag:         "../invalid",
			description: "Invalid checkpoint",
			wantErr:     true,
		},
		{
			name:        "Create duplicate checkpoint",
			tag:         "test-checkpoint",
			description: "Duplicate checkpoint",
			wantErr:     true,
			errType:     ErrCheckpointExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := manager.Create(ctx, tt.tag, tt.description)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errType != nil {
					assert.ErrorIs(t, err, tt.errType)
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, info)

			if tt.tag != "" {
				assert.Equal(t, tt.tag, info.ID)
			} else {
				assert.Contains(t, info.ID, "checkpoint-")
			}

			assert.Equal(t, tt.description, info.Description)
			assert.Greater(t, info.FileSize, int64(0))
			assert.Equal(t, 3, info.Transactions)
			assert.Equal(t, 2, info.Vendors)
			assert.Equal(t, 3, info.Categories)
			assert.Equal(t, 7, info.SchemaVersion)
			assert.False(t, info.IsAuto)

			// Verify checkpoint file exists
			checkpointPath := filepath.Join(filepath.Dir(dbPath), "checkpoints", info.ID+".db")
			_, err = os.Stat(checkpointPath)
			assert.NoError(t, err)

			// Verify metadata file exists
			metadataPath := filepath.Join(filepath.Dir(dbPath), "checkpoints", info.ID+".meta.json")
			_, err = os.Stat(metadataPath)
			assert.NoError(t, err)
		})
	}
}

func TestCheckpointManager_List(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	manager, err := NewCheckpointManager(db, dbPath)
	require.NoError(t, err)

	ctx := context.Background()

	// Create multiple checkpoints
	_, err = manager.Create(ctx, "checkpoint-1", "First checkpoint")
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond) // Ensure different timestamps

	_, err = manager.Create(ctx, "checkpoint-2", "Second checkpoint")
	require.NoError(t, err)

	// List checkpoints
	checkpoints, err := manager.List(ctx)
	require.NoError(t, err)

	assert.Len(t, checkpoints, 2)

	// Should be sorted by creation time (newest first)
	assert.Equal(t, "checkpoint-2", checkpoints[0].ID)
	assert.Equal(t, "checkpoint-1", checkpoints[1].ID)

	assert.Equal(t, "Second checkpoint", checkpoints[0].Description)
	assert.Equal(t, "First checkpoint", checkpoints[1].Description)
}

func TestCheckpointManager_Restore(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	insertTestData(t, db)

	manager, err := NewCheckpointManager(db, dbPath)
	require.NoError(t, err)

	ctx := context.Background()

	// Create a checkpoint
	_, err = manager.Create(ctx, "restore-test", "Checkpoint for restore test")
	require.NoError(t, err)

	// Modify the database
	_, err = db.Exec("DELETE FROM transactions WHERE id = 'tx1'")
	require.NoError(t, err)

	// Verify transaction was deleted
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Close DB before restore
	db.Close()

	// Restore checkpoint
	err = manager.Restore(ctx, "restore-test")
	require.NoError(t, err)

	// Reopen database to verify restore
	db, err = sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Verify transaction was restored
	err = db.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Test restore non-existent checkpoint
	err = manager.Restore(ctx, "non-existent")
	assert.ErrorIs(t, err, ErrCheckpointNotFound)
}

func TestCheckpointManager_Delete(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	manager, err := NewCheckpointManager(db, dbPath)
	require.NoError(t, err)

	ctx := context.Background()

	// Create a checkpoint
	_, err = manager.Create(ctx, "delete-test", "Checkpoint for delete test")
	require.NoError(t, err)

	// Verify checkpoint exists
	checkpoints, err := manager.List(ctx)
	require.NoError(t, err)
	assert.Len(t, checkpoints, 1)

	// Delete checkpoint
	err = manager.Delete(ctx, "delete-test")
	require.NoError(t, err)

	// Verify checkpoint was deleted
	checkpoints, err = manager.List(ctx)
	require.NoError(t, err)
	assert.Len(t, checkpoints, 0)

	// Verify files were deleted
	checkpointPath := filepath.Join(filepath.Dir(dbPath), "checkpoints", "delete-test.db")
	_, err = os.Stat(checkpointPath)
	assert.True(t, os.IsNotExist(err))

	// Test delete non-existent checkpoint
	err = manager.Delete(ctx, "non-existent")
	assert.ErrorIs(t, err, ErrCheckpointNotFound)
}

func TestCheckpointManager_AutoCheckpoint(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	manager, err := NewCheckpointManager(db, dbPath)
	require.NoError(t, err)

	ctx := context.Background()

	// Create auto checkpoint
	err = manager.AutoCheckpoint(ctx, "import")
	require.NoError(t, err)

	// Verify auto checkpoint was created
	checkpoints, err := manager.List(ctx)
	require.NoError(t, err)
	assert.Len(t, checkpoints, 1)
	assert.True(t, checkpoints[0].IsAuto)
	assert.Contains(t, checkpoints[0].ID, "auto-import-")
	assert.Contains(t, checkpoints[0].Description, "Automatic checkpoint before import")
}

func TestCheckpointManager_IntegrityCheck(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	manager, err := NewCheckpointManager(db, dbPath)
	require.NoError(t, err)

	ctx := context.Background()

	// Create a checkpoint
	_, err = manager.Create(ctx, "integrity-test", "Checkpoint for integrity test")
	require.NoError(t, err)

	// Corrupt the checkpoint file
	checkpointPath := filepath.Join(filepath.Dir(dbPath), "checkpoints", "integrity-test.db")

	// Write garbage to the file
	err = os.WriteFile(checkpointPath, []byte("corrupted data"), 0644)
	require.NoError(t, err)

	// Attempt to restore corrupted checkpoint
	err = manager.Restore(ctx, "integrity-test")
	assert.ErrorIs(t, err, ErrCheckpointCorrupted)
}

func TestCheckpointManager_CollectRowCounts(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	insertTestData(t, db)

	manager, err := NewCheckpointManager(db, dbPath)
	require.NoError(t, err)

	ctx := context.Background()

	rowCounts, err := manager.collectRowCounts(ctx)
	require.NoError(t, err)

	assert.Equal(t, 3, rowCounts["transactions"])
	assert.Equal(t, 2, rowCounts["vendors"])
	assert.Equal(t, 3, rowCounts["categories"])
	assert.Equal(t, 0, rowCounts["classifications"])
	assert.Equal(t, 0, rowCounts["progress"])
}

func TestCheckpointManager_CleanupOldAutoCheckpoints(t *testing.T) {
	db, dbPath, cleanup := setupTestDB(t)
	defer cleanup()

	manager, err := NewCheckpointManager(db, dbPath)
	require.NoError(t, err)

	ctx := context.Background()

	// Create multiple auto checkpoints
	for i := 0; i < 7; i++ {
		err = manager.AutoCheckpoint(ctx, fmt.Sprintf("test-%d", i))
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond) // Ensure different timestamps
	}

	// List all checkpoints
	checkpoints, err := manager.List(ctx)
	require.NoError(t, err)

	// Should have only 5 auto checkpoints (cleanup keeps 5 most recent)
	autoCount := 0
	for _, cp := range checkpoints {
		if cp.IsAuto {
			autoCount++
		}
	}
	assert.Equal(t, 5, autoCount)
}
