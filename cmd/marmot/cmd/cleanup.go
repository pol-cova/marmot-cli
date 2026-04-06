package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/spf13/cobra"
)

func newCleanupCmd() *cobra.Command {
	var dryRun bool
	var force bool

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up old local backups",
		Long: `Remove local backup files based on retention policy.

For local-only storage mode, this command removes backups older than
the configured retention period. Use --dry-run to preview what would
be deleted without actually removing files.

Examples:
  # Preview cleanup (no files deleted)
  marmot cleanup --dry-run

  # Clean up old backups
  marmot cleanup

  # Skip confirmation
  marmot cleanup --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCleanup(dryRun, force)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be deleted without deleting")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation prompt")

	return cmd
}

func runCleanup(dryRun, force bool) error {
	cfg, err := config.LoadConfig(getConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if using local storage
	if !cfg.IsLocal() {
		return fmt.Errorf("cleanup is only for local storage mode (current: cloud storage)")
	}

	// Check if retention is configured
	if cfg.Local.RetentionDays <= 0 {
		return fmt.Errorf("retention not configured (retention_days: %d)", cfg.Local.RetentionDays)
	}

	backupPath := cfg.GetStoragePath()

	// List all backup files
	files, err := listBackupFiles(backupPath)
	if err != nil {
		return fmt.Errorf("failed to list backup files: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No backup files found.")
		return nil
	}

	// Calculate cutoff date
	retentionDays := cfg.Local.RetentionDays
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	fmt.Printf("Retention policy: %d days\n", retentionDays)
	fmt.Printf("Cutoff date: %s\n", cutoffTime.Format("2006-01-02"))
	fmt.Printf("Scanning: %s\n", backupPath)
	fmt.Println()

	// Find files to delete
	var toDelete []backupFileInfo
	var toKeep int

	for _, file := range files {
		if file.ModTime.Before(cutoffTime) {
			toDelete = append(toDelete, file)
		} else {
			toKeep++
		}
	}

	if len(toDelete) == 0 {
		fmt.Printf("No backups older than %d days found.\n", retentionDays)
		fmt.Printf("Total backups: %d (all within retention period)\n", toKeep)
		return nil
	}

	// Calculate total size to free
	var totalSize int64
	for _, file := range toDelete {
		totalSize += file.Size
	}

	// Show summary
	fmt.Printf("Found %d backups to delete:\n", len(toDelete))
	for _, file := range toDelete {
		fmt.Printf("  - %s (%s, %s)\n",
			file.Name,
			file.ModTime.Format("2006-01-02 15:04"),
			humanReadableSize(file.Size))
	}
	fmt.Println()
	fmt.Printf("Total space to free: %s\n", humanReadableSize(totalSize))
	fmt.Printf("Backups to keep: %d\n", toKeep)
	fmt.Println()

	if dryRun {
		fmt.Println("[DRY-RUN] No files were deleted. Run without --dry-run to clean up.")
		return nil
	}

	// Confirm deletion
	if !force {
		fmt.Printf("Delete %d backup files? (y/n): ", len(toDelete))
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Cleanup cancelled.")
			return nil
		}
		fmt.Println()
	}

	// Delete files
	deleted := 0
	failed := 0
	var freedSpace int64

	for _, file := range toDelete {
		path := filepath.Join(backupPath, file.Name)
		if err := os.Remove(path); err != nil {
			fmt.Printf("Failed to delete %s: %v\n", file.Name, err)
			failed++
		} else {
			fmt.Printf("Deleted: %s\n", file.Name)
			deleted++
			freedSpace += file.Size
		}
	}

	fmt.Println()
	fmt.Printf("Cleanup complete: %d deleted, %d failed\n", deleted, failed)
	fmt.Printf("Space freed: %s\n", humanReadableSize(freedSpace))

	return nil
}

type backupFileInfo struct {
	Name    string
	ModTime time.Time
	Size    int64
}

func listBackupFiles(path string) ([]backupFileInfo, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []backupFileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Only count .enc files (encrypted backups)
		if !strings.HasSuffix(entry.Name(), ".enc") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, backupFileInfo{
			Name:    entry.Name(),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		})
	}

	return files, nil
}

func humanReadableSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
