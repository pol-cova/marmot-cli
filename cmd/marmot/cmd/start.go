package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pol-cova/marmot-cli/internal/agent"
	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/pol-cova/marmot-cli/internal/crypto"
	"github.com/pol-cova/marmot-cli/internal/remote"
	"github.com/pol-cova/marmot-cli/internal/storage"

	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	var foreground bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Marmot daemon",
		Long:  "Starts the Marmot daemon to run scheduled backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart(cmd, foreground)
		},
	}

	cmd.Flags().BoolVar(&foreground, "foreground", false, "run in foreground (don't daemonize)")

	return cmd
}

func runStart(cmd *cobra.Command, foreground bool) error {
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

	// Create scheduler
	scheduler := agent.NewScheduler(ag, cfg)

	// Start scheduler
	if err := scheduler.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	fmt.Println("Marmot daemon started")

	if cfg.IsLocal() {
		fmt.Println("Storage: Local Only")
		fmt.Printf("Backup path: %s\n", cfg.GetStoragePath())
		if cfg.Local.RetentionDays > 0 {
			fmt.Printf("Retention: %d days\n", cfg.Local.RetentionDays)
		}
	} else {
		fmt.Printf("Storage: Cloud (%s)\n", cfg.S3.Provider)
		fmt.Printf("Bucket: %s\n", cfg.S3.Bucket)
	}

	fmt.Println("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan

	fmt.Println("\nShutting down...")
	scheduler.Stop()

	return nil
}
