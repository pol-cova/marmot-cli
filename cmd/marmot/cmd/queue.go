package cmd

import (
	"fmt"

	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/pol-cova/marmot-cli/internal/storage"

	"github.com/spf13/cobra"
)

func newQueueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Manage upload queue",
		Long:  "Commands to manage the upload queue",
	}

	cmd.AddCommand(newQueueClearCmd())
	cmd.AddCommand(newQueueListCmd())

	return cmd
}

func newQueueClearCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear all items from the upload queue",
		Long:  "Removes all items from the upload queue",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(getConfigPath())
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			queue, err := storage.NewSQLiteQueue(cfg.Paths.StateDB)
			if err != nil {
				return fmt.Errorf("failed to open queue: %w", err)
			}
			defer queue.Close()

			if err := queue.Clear(); err != nil {
				return fmt.Errorf("failed to clear queue: %w", err)
			}

			fmt.Println("Upload queue cleared successfully")
			return nil
		},
	}

	return cmd
}

func newQueueListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all items in the upload queue",
		Long:  "Displays all items currently in the upload queue",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(getConfigPath())
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			queue, err := storage.NewSQLiteQueue(cfg.Paths.StateDB)
			if err != nil {
				return fmt.Errorf("failed to open queue: %w", err)
			}
			defer queue.Close()

			items, err := queue.List()
			if err != nil {
				return fmt.Errorf("failed to list queue: %w", err)
			}

			if len(items) == 0 {
				fmt.Println("Upload queue is empty")
				return nil
			}

			fmt.Printf("Upload queue (%d items):\n\n", len(items))
			for i, item := range items {
				fmt.Printf("%d. %s\n", i+1, item.Name)
				fmt.Printf("   Database: %s\n", item.DatabaseID)
				fmt.Printf("   Size: %d bytes\n", item.Size)
				fmt.Printf("   Created: %s\n", item.CreatedAt.Format("2006-01-02 15:04:05"))
				if i < len(items)-1 {
					fmt.Println()
				}
			}

			return nil
		},
	}

	return cmd
}
