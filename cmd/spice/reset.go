package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset all transaction classifications",
	Long: `Reset removes all transaction classifications, allowing transactions to be re-classified.

This is a destructive operation that will delete all existing classifications and vendor rules.
Classification history will be preserved for auditing purposes.`,
	RunE: runReset,
}

var (
	resetForce      bool
	resetKeepVendor bool
)

func init() {
	rootCmd.AddCommand(resetCmd)
	resetCmd.Flags().BoolVarP(&resetForce, "force", "f", false, "Skip confirmation prompt")
	resetCmd.Flags().BoolVar(&resetKeepVendor, "keep-vendors", false, "Keep vendor rules (only reset classifications)")
}

func runReset(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	// Get database
	store, cleanup, err := getDatabase()
	if err != nil {
		return err
	}
	defer cleanup()

	// Get count of existing classifications
	classificationCount, err := getClassificationCount(ctx, store)
	if err != nil {
		return fmt.Errorf("failed to count classifications: %w", err)
	}

	if classificationCount == 0 {
		if _, err := fmt.Fprintf(os.Stdout, "No classifications found. Nothing to reset.\n"); err != nil {
			slog.Error("failed to write output", "error", err)
		}
		return nil
	}

	// Confirm with user unless --force is used
	if !resetForce {
		if _, err := fmt.Fprintf(os.Stdout, "This will delete %d transaction classifications.\n", classificationCount); err != nil {
			slog.Error("failed to write output", "error", err)
		}
		if !resetKeepVendor {
			vendorCount, _ := getVendorCount(ctx, store)
			if vendorCount > 0 {
				if _, err := fmt.Fprintf(os.Stdout, "This will also delete %d vendor rules.\n", vendorCount); err != nil {
					slog.Error("failed to write output", "error", err)
				}
			} else if vendorCount == -1 {
				if _, err := fmt.Fprintf(os.Stdout, "This will also delete all vendor rules.\n"); err != nil {
					slog.Error("failed to write output", "error", err)
				}
			}
		}
		if _, err := fmt.Fprintf(os.Stdout, "\nAre you sure you want to continue? [y/N]: "); err != nil {
			slog.Error("failed to write output", "error", err)
		}

		var response string
		if _, err := fmt.Scanln(&response); err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		if response != "y" && response != "Y" {
			if _, err := fmt.Fprintf(os.Stdout, "Reset canceled.\n"); err != nil {
				slog.Error("failed to write output", "error", err)
			}
			return nil
		}
	}

	// Clear classifications
	if err := clearClassifications(ctx, store); err != nil {
		return fmt.Errorf("failed to clear classifications: %w", err)
	}

	// Clear vendor rules if requested
	if !resetKeepVendor {
		if err := clearVendors(ctx, store); err != nil {
			return fmt.Errorf("failed to clear vendors: %w", err)
		}
	}

	// Print summary
	if _, err := fmt.Fprintf(os.Stdout, "✅ Successfully reset %d classifications", classificationCount); err != nil {
		slog.Error("failed to write output", "error", err)
	}
	if !resetKeepVendor {
		vendorCount, _ := getVendorCount(ctx, store)
		if vendorCount > 0 {
			if _, err := fmt.Fprintf(os.Stdout, " and deleted vendor rules"); err != nil {
				slog.Error("failed to write output", "error", err)
			}
		}
	}
	if _, err := fmt.Fprintf(os.Stdout, "\n\nTransactions are now ready to be re-classified. Run 'spice classify' to start.\n"); err != nil {
		slog.Error("failed to write output", "error", err)
	}

	return nil
}

func getClassificationCount(ctx context.Context, store *storage.SQLiteStorage) (int, error) {
	// Get all classifications using a wide date range
	startDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

	classifications, err := store.GetClassificationsByDateRange(ctx, startDate, endDate)
	if err != nil {
		return 0, err
	}

	// Count non-empty classifications
	count := 0
	for _, c := range classifications {
		if c.Category != "" {
			count++
		}
	}
	return count, nil
}

func getVendorCount(_ context.Context, _ *storage.SQLiteStorage) (int, error) {
	// Get all vendors by using GetVendors method if available,
	// or count them indirectly through transactions
	// For now, we'll return -1 to indicate vendor count is not available
	// but the reset will still work
	return -1, nil
}

func clearClassifications(ctx context.Context, store *storage.SQLiteStorage) error {
	// Use the built-in method to clear all classifications
	return store.ClearAllClassifications(ctx)
}

func clearVendors(ctx context.Context, store *storage.SQLiteStorage) error {
	// Delete all vendor rules by source
	// First delete auto-generated vendors
	if err := store.DeleteVendorsBySource(ctx, model.SourceAuto); err != nil {
		return fmt.Errorf("failed to delete auto vendors: %w", err)
	}
	// Then delete confirmed vendors
	if err := store.DeleteVendorsBySource(ctx, model.SourceAutoConfirmed); err != nil {
		return fmt.Errorf("failed to delete confirmed vendors: %w", err)
	}
	// Finally delete manually-created vendors
	if err := store.DeleteVendorsBySource(ctx, model.SourceManual); err != nil {
		return fmt.Errorf("failed to delete manual vendors: %w", err)
	}
	return nil
}
