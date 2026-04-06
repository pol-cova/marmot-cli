package cmd

import (
	"fmt"
	"os"

	"github.com/pol-cova/marmot-cli/internal/agent"
	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/pol-cova/marmot-cli/internal/crypto"
	"github.com/pol-cova/marmot-cli/internal/remote"
	"github.com/pol-cova/marmot-cli/internal/storage"

	"github.com/spf13/cobra"
)

func newBackupCmd() *cobra.Command {
	var allFlag bool
	var databaseID string

	cmd := &cobra.Command{
		Use:   "backup [database-id]",
		Short: "Perform a manual backup",
		Long:  "Backs up a specific database or all databases if --all is used",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBackup(cmd, allFlag, databaseID)
		},
	}

	cmd.Flags().BoolVar(&allFlag, "all", false, "backup all databases")
	cmd.Flags().StringVar(&databaseID, "db", "", "database ID to backup")

	return cmd
}

func runBackup(cmd *cobra.Command, allFlag bool, databaseID string) error {
	cfg, err := config.LoadConfig(getConfigPath())
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
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

	encryptor := crypto.NewAESEncryptor()
	if err := encryptor.LoadKeyFromFile(cfg.Paths.KeyFile); err != nil {
		return fmt.Errorf("failed to load encryption key: %w", err)
	}

	// Create agent
	ag := agent.NewAgent(cfg, stor, queue, remoteStorage, encryptor)

	// Determine which databases to backup
	var databases []*config.DatabaseConfig

	if allFlag {
		for i := range cfg.Databases {
			if cfg.Databases[i].Enabled {
				databases = append(databases, &cfg.Databases[i])
			}
		}
	} else if databaseID != "" {
		db := cfg.GetDatabaseByID(databaseID)
		if db == nil {
			return fmt.Errorf("database not found: %s", databaseID)
		}
		databases = append(databases, db)
	} else {
		return fmt.Errorf("must specify --all or --db <database-id>")
	}

	// Perform backups
	for _, db := range databases {
		fmt.Printf("Backing up database: %s (%s)\n", db.ID, db.Name)

		// For local-only: waitForUpload=true completes immediately after local save
		// For cloud: waitForUpload=false queues for async upload
		waitForUpload := cfg.IsS3()
		if err := ag.Backup(cmd.Context(), db, waitForUpload); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to backup %s: %v\n", db.ID, err)
			continue
		}

		if cfg.IsLocal() {
			fmt.Printf("Backup completed (local): %s\n", db.ID)
		} else {
			fmt.Printf("Backup saved locally and queued for upload: %s\n", db.ID)
		}
	}

	if cfg.IsS3() {
		fmt.Println("\nRun 'marmot status' to check upload progress.")
	} else {
		fmt.Println("\nRun 'marmot cleanup' to manage retention.")
	}

	return nil
}
