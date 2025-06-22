package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/cli"
	"github.com/Veraticus/the-spice-must-flow/internal/storage"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

func checkpointCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkpoint",
		Short: "Manage database checkpoints",
		Long: `Create, list, restore, and delete database checkpoints.
		
Checkpoints allow you to save the current state of your database before making
risky changes, and restore to a previous state if needed.`,
		Example: `  # Create a checkpoint before importing new data
  spice checkpoint create --tag "pre-2024-import"
  
  # List all checkpoints
  spice checkpoint list
  
  # Restore from a checkpoint
  spice checkpoint restore pre-2024-import
  
  # Delete an old checkpoint
  spice checkpoint delete old-checkpoint`,
	}

	cmd.AddCommand(createCheckpointCmd())
	cmd.AddCommand(listCheckpointsCmd())
	cmd.AddCommand(restoreCheckpointCmd())
	cmd.AddCommand(deleteCheckpointCmd())

	return cmd
}

func createCheckpointCmd() *cobra.Command {
	var tag string
	var description string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new checkpoint",
		Long:  `Create a snapshot of the current database state.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.Background()

			// Initialize storage with auto-migration
			store, err := initStorage(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := store.Close(); closeErr != nil {
					slog.Error("failed to close storage", "error", closeErr)
				}
			}()

			// Create checkpoint manager
			sqliteStore, ok := store.(*storage.SQLiteStorage)
			if !ok {
				return fmt.Errorf("storage is not SQLite")
			}
			manager, err := sqliteStore.NewCheckpointManager()
			if err != nil {
				return fmt.Errorf("failed to create checkpoint manager: %w", err)
			}

			// Create checkpoint
			info, err := manager.Create(ctx, tag, description)
			if err != nil {
				return fmt.Errorf("failed to create checkpoint: %w", err)
			}

			// Format output
			if _, err := fmt.Fprintf(os.Stdout, "%s Created checkpoint %s (%s)\n",
				cli.SuccessStyle.Render("✓"),
				cli.InfoStyle.Render(info.ID),
				formatFileSize(info.FileSize)); err != nil {
				slog.Error("failed to write output", "error", err)
			}

			if info.Description != "" {
				if _, err := fmt.Fprintf(os.Stdout, "  Description: %s\n", info.Description); err != nil {
					slog.Error("failed to write output", "error", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Checkpoint tag/name (auto-generated if not provided)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Description of the checkpoint")

	return cmd
}

func listCheckpointsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all checkpoints",
		Long:  `Display all available checkpoints with their metadata.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx := context.Background()

			// Initialize storage with auto-migration
			store, err := initStorage(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := store.Close(); closeErr != nil {
					slog.Error("failed to close storage", "error", closeErr)
				}
			}()

			// Create checkpoint manager
			sqliteStore, ok := store.(*storage.SQLiteStorage)
			if !ok {
				return fmt.Errorf("storage is not SQLite")
			}
			manager, err := sqliteStore.NewCheckpointManager()
			if err != nil {
				return fmt.Errorf("failed to create checkpoint manager: %w", err)
			}

			// List checkpoints
			checkpoints, err := manager.List(ctx)
			if err != nil {
				return fmt.Errorf("failed to list checkpoints: %w", err)
			}

			if len(checkpoints) == 0 {
				if _, err := fmt.Fprintln(os.Stdout, cli.SubtitleStyle.Render("No checkpoints found.")); err != nil {
					slog.Error("failed to write output", "error", err)
				}
				return nil
			}

			// Create table
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

			// Header
			headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
			if _, err := fmt.Fprintln(w, strings.Join([]string{
				headerStyle.Render("NAME"),
				headerStyle.Render("CREATED"),
				headerStyle.Render("SIZE"),
				headerStyle.Render("TRANSACTIONS"),
				headerStyle.Render("CATEGORIES"),
				headerStyle.Render("TYPE"),
			}, "\t")); err != nil {
				slog.Error("failed to write table header", "error", err)
			}

			// Rows
			for _, cp := range checkpoints {
				typeLabel := "manual"
				if cp.IsAuto {
					typeLabel = "auto"
				}

				if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\n",
					cli.InfoStyle.Render(cp.ID),
					formatRelativeTime(cp.CreatedAt),
					formatFileSize(cp.FileSize),
					cp.Transactions,
					cp.Categories,
					cli.SubtitleStyle.Render(typeLabel),
				); err != nil {
					slog.Error("failed to write checkpoint row", "error", err)
				}
			}

			if err := w.Flush(); err != nil {
				slog.Error("failed to flush table writer", "error", err)
			}

			return nil
		},
	}
}

func restoreCheckpointCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "restore <checkpoint-id>",
		Short: "Restore database from a checkpoint",
		Long:  `Replace the current database with a checkpoint.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()
			checkpointID := args[0]

			// Initialize storage with auto-migration
			store, err := initStorage(ctx)
			if err != nil {
				return err
			}

			// Create checkpoint manager
			sqliteStore, ok := store.(*storage.SQLiteStorage)
			if !ok {
				if closeErr := store.Close(); closeErr != nil {
					slog.Error("failed to close storage", "error", closeErr)
				}
				return fmt.Errorf("storage is not SQLite")
			}
			manager, err := sqliteStore.NewCheckpointManager()
			if err != nil {
				if closeErr := store.Close(); closeErr != nil {
					slog.Error("failed to close storage", "error", closeErr)
				}
				return fmt.Errorf("failed to create checkpoint manager: %w", err)
			}

			// Get checkpoint info
			info, err := manager.GetCheckpointInfo(ctx, checkpointID)
			if err != nil {
				if closeErr := store.Close(); closeErr != nil {
					slog.Error("failed to close storage", "error", closeErr)
				}
				return fmt.Errorf("failed to get checkpoint info: %w", err)
			}

			// Confirm unless force flag is set
			if !force {
				if _, writeErr := fmt.Fprintf(os.Stdout, "%s This will replace your current database with checkpoint %s.\n",
					cli.WarningStyle.Render("⚠️"),
					cli.InfoStyle.Render(checkpointID)); writeErr != nil {
					slog.Error("failed to write output", "error", writeErr)
				}
				if _, writeErr := fmt.Fprintf(os.Stdout, "  Created: %s\n", info.CreatedAt.Format("2006-01-02 15:04:05")); writeErr != nil {
					slog.Error("failed to write output", "error", writeErr)
				}
				if info.Description != "" {
					if _, writeErr := fmt.Fprintf(os.Stdout, "  Description: %s\n", info.Description); writeErr != nil {
						slog.Error("failed to write output", "error", writeErr)
					}
				}
				if _, writeErr := fmt.Fprintf(os.Stdout, "\nContinue? (y/N) "); writeErr != nil {
					slog.Error("failed to write output", "error", writeErr)
				}

				var response string
				if _, scanErr := fmt.Scanln(&response); scanErr != nil {
					// EOF or empty input is treated as "N"
					response = "n"
				}
				if !strings.HasPrefix(strings.ToLower(response), "y") {
					if _, writeErr := fmt.Fprintln(os.Stdout, cli.SubtitleStyle.Render("Restore canceled.")); writeErr != nil {
						slog.Error("failed to write output", "error", writeErr)
					}
					if closeErr := store.Close(); closeErr != nil {
						slog.Error("failed to close storage", "error", closeErr)
					}
					return nil
				}
			}

			// Must close storage before restore
			if closeErr := store.Close(); closeErr != nil {
				slog.Error("failed to close storage before restore", "error", closeErr)
			}

			// Restore checkpoint
			// Note: We need to recreate the manager after closing storage
			store, err = initStorage(ctx)
			if err != nil {
				return err
			}
			sqliteStore2, ok2 := store.(*storage.SQLiteStorage)
			if !ok2 {
				if closeErr := store.Close(); closeErr != nil {
					slog.Error("failed to close storage", "error", closeErr)
				}
				return fmt.Errorf("storage is not SQLite")
			}
			manager, err = sqliteStore2.NewCheckpointManager()
			if err != nil {
				if closeErr := store.Close(); closeErr != nil {
					slog.Error("failed to close storage", "error", closeErr)
				}
				return fmt.Errorf("failed to recreate checkpoint manager: %w", err)
			}

			if err := manager.Restore(ctx, checkpointID); err != nil {
				if closeErr := store.Close(); closeErr != nil {
					slog.Error("failed to close storage", "error", closeErr)
				}
				return fmt.Errorf("failed to restore checkpoint: %w", err)
			}

			if _, err := fmt.Fprintf(os.Stdout, "%s Restored from checkpoint %s\n",
				cli.SuccessStyle.Render("✓"),
				cli.InfoStyle.Render(checkpointID)); err != nil {
				slog.Error("failed to write output", "error", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

func deleteCheckpointCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <checkpoint-id>",
		Short: "Delete a checkpoint",
		Long:  `Permanently remove a checkpoint.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ctx := context.Background()
			checkpointID := args[0]

			// Initialize storage with auto-migration
			store, err := initStorage(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if closeErr := store.Close(); closeErr != nil {
					slog.Error("failed to close storage", "error", closeErr)
				}
			}()

			// Create checkpoint manager
			sqliteStore, ok := store.(*storage.SQLiteStorage)
			if !ok {
				return fmt.Errorf("storage is not SQLite")
			}
			manager, err := sqliteStore.NewCheckpointManager()
			if err != nil {
				return fmt.Errorf("failed to create checkpoint manager: %w", err)
			}

			// Get checkpoint info
			info, err := manager.GetCheckpointInfo(ctx, checkpointID)
			if err != nil {
				return fmt.Errorf("failed to get checkpoint info: %w", err)
			}

			// Confirm unless force flag is set
			if !force {
				if _, err := fmt.Fprintf(os.Stdout, "%s This will permanently delete checkpoint %s.\n",
					cli.WarningStyle.Render("⚠️"),
					cli.InfoStyle.Render(checkpointID)); err != nil {
					slog.Error("failed to write output", "error", err)
				}
				if _, err := fmt.Fprintf(os.Stdout, "  Created: %s\n", info.CreatedAt.Format("2006-01-02 15:04:05")); err != nil {
					slog.Error("failed to write output", "error", err)
				}
				if _, err := fmt.Fprintf(os.Stdout, "  Size: %s\n", formatFileSize(info.FileSize)); err != nil {
					slog.Error("failed to write output", "error", err)
				}
				if _, err := fmt.Fprintf(os.Stdout, "\nContinue? (y/N) "); err != nil {
					slog.Error("failed to write output", "error", err)
				}

				var response string
				if _, err := fmt.Scanln(&response); err != nil {
					// EOF or empty input is treated as "N"
					response = "n"
				}
				if !strings.HasPrefix(strings.ToLower(response), "y") {
					if _, err := fmt.Fprintln(os.Stdout, cli.SubtitleStyle.Render("Deletion canceled.")); err != nil {
						slog.Error("failed to write output", "error", err)
					}
					return nil
				}
			}

			// Delete checkpoint
			if err := manager.Delete(ctx, checkpointID); err != nil {
				return fmt.Errorf("failed to delete checkpoint: %w", err)
			}

			if _, err := fmt.Fprintf(os.Stdout, "%s Deleted checkpoint %s\n",
				cli.SuccessStyle.Render("✓"),
				cli.InfoStyle.Render(checkpointID)); err != nil {
				slog.Error("failed to write output", "error", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

// Helper functions

func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func formatRelativeTime(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case duration < 7*24*time.Hour:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("2006-01-02 15:04")
	}
}
