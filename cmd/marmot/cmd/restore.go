package cmd

import (
	"fmt"

	"github.com/pol-cova/marmot-cli/internal/agent"
	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/pol-cova/marmot-cli/internal/crypto"
	"github.com/pol-cova/marmot-cli/internal/remote"
	"github.com/pol-cova/marmot-cli/internal/storage"

	"github.com/spf13/cobra"
)

func newRestoreCmd() *cobra.Command {
	var (
		databaseID string
		backupID   string
		encFile    string
		force      bool
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore a database backup",
		Long:  "Restore a backup either from the remote storage by backup ID or from a local encrypted .enc file",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRestore(cmd, databaseID, backupID, encFile, force, dryRun)
		},
	}

	cmd.Flags().StringVar(&databaseID, "db", "", "database ID to restore to (required)")
	cmd.Flags().StringVar(&backupID, "backup", "", "backup ID to restore (default: latest for cloud, required for local)")
	cmd.Flags().StringVar(&encFile, "file", "", "path to a local encrypted .enc backup file to restore from")
	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation prompt")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate backup without actually restoring (integrity check only)")

	cmd.MarkFlagRequired("db")

	return cmd
}

func runRestore(cmd *cobra.Command, databaseID, backupID, encFile string, force, dryRun bool) error {
	cfg, err := config.LoadConfig(getConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	dbConfig := cfg.GetDatabaseByID(databaseID)
	if dbConfig == nil {
		return fmt.Errorf("database not found: %s", databaseID)
	}

	// Initialize components
	stor, err := storage.NewLocalStorage(cfg.Paths.BackupsDir)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	queue, err := storage.NewSQLiteQueue(cfg.Paths.StateDB)
	if err != nil {
		return fmt.Errorf("failed to create queue: %w", err)
	}
	defer queue.Close()

	// Create remote storage only for cloud mode
	var remoteStorage *remote.Storage
	if cfg.IsS3() {
		rs, err := remote.NewStorageWithContext(cmd.Context(), cfg)
		if err != nil {
			return fmt.Errorf("failed to create remote storage: %w", err)
		}
		remoteStorage = rs
	}

	// For local mode without --file flag, list local backups
	if cfg.IsLocal() && encFile == "" {
		return fmt.Errorf("local storage mode: use --file to specify a local backup file, or run 'marmot restore --db %s --file /path/to/backup.enc'", databaseID)
	}

	encryptor := crypto.NewAESEncryptor()
	if err := encryptor.LoadKeyFromFile(cfg.Paths.KeyFile); err != nil {
		return fmt.Errorf("failed to load encryption key: %w", err)
	}

	ag := agent.NewAgent(cfg, stor, queue, remoteStorage, encryptor)

	// Determine source and confirm
	var actionDesc string
	if encFile != "" {
		actionDesc = fmt.Sprintf("file %s", encFile)
	} else {
		if backupID == "" {
			if remoteStorage == nil {
				return fmt.Errorf("no remote storage configured and no --file specified")
			}
			backups, err := remoteStorage.GetClient().ListBackups(cmd.Context(), dbConfig.Name, 10)
			if err != nil {
				return fmt.Errorf("failed to list backups: %w", err)
			}
			if len(backups) == 0 {
				return fmt.Errorf("no backups found for database: %s", dbConfig.Name)
			}
			backupID = backups[0].BackupID
			fmt.Printf("Using latest backup: %s\n", backupID)
		}
		actionDesc = fmt.Sprintf("backup %s", backupID)
	}

	if !force {
		fmt.Printf("This will restore %s to database %s.\n", actionDesc, databaseID)
		fmt.Print("This operation may overwrite existing data. Continue? (y/n): ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "yes" {
			return fmt.Errorf("restore cancelled")
		}
	}

	if dryRun {
		fmt.Printf("\n[DRY-RUN] Validating %s without restoring...\n", actionDesc)
		fmt.Println("This will download, decrypt, and verify the backup integrity.")
		fmt.Println()

		// If dry-run with local file, just verify it
		if encFile != "" {
			// Re-use verify logic by calling verify command
			return runVerify(cmd, "", "", false, encFile, "detailed")
		}

		// For remote backups, download then verify
		return runVerify(cmd, backupID, "", false, "", "detailed")
	}

	// Execute restore
	if encFile != "" {
		fmt.Printf("Restoring from %s to database %s...\n", encFile, databaseID)
		if err := ag.RestoreFromFile(cmd.Context(), dbConfig, encFile); err != nil {
			return fmt.Errorf("restore failed: %w", err)
		}
	} else {
		fmt.Printf("Restoring backup %s to database %s...\n", backupID, databaseID)
		if err := ag.Restore(cmd.Context(), dbConfig, backupID); err != nil {
			return fmt.Errorf("restore failed: %w", err)
		}
	}

	fmt.Println("Restore completed successfully!")
	return nil
}
