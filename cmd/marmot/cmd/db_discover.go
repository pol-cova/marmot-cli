package cmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/pol-cova/marmot-cli/internal/config"
	"github.com/pol-cova/marmot-cli/internal/discovery"

	"github.com/spf13/cobra"
)

func newDbDiscoverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "discover",
		Short: "Discover databases on this host",
		Long:  "Scans Docker containers and local services for databases, then adds selected ones to config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(getConfigPath())
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			reader := bufio.NewReader(os.Stdin)

			fmt.Println("\n=== Database Discovery ===")
			fmt.Println("Scanning Docker containers for databases...")

			var databases []discovery.DatabaseInfo
			discoverer, err := discovery.NewDockerDiscoverer()
			if err != nil {
				fmt.Printf("Warning: Docker not available (%v). Skipping auto-discovery.\n", err)
			} else {
				databases, err = discoverer.Discover(cmd.Context())
				if err != nil {
					fmt.Printf("Warning: Failed to discover databases: %v\n", err)
				}
			}

			if err := mergeLocalDatabases(cmd.Context(), &databases); err != nil {
				fmt.Printf("Warning: Failed to discover local databases: %v\n", err)
			}

			if len(databases) == 0 {
				fmt.Println("No databases found.")
				return nil
			}

			var added int
			for _, db := range databases {
				dbConfig, ok := buildDbConfigFromDiscovery(reader, db)
				if !ok {
					continue
				}

				status, err := verifyDiscoveredConnection(cmd.Context(), dbConfig)
				if err != nil {
					fmt.Printf("Connection check: failed (%v)\n", err)
				} else {
					fmt.Printf("Connection check: ok (%s)\n", status)
				}

				if cfg.GetDatabaseByID(dbConfig.ID) != nil {
					dbConfig.ID = prompt(reader, "Database ID already exists; enter a new ID", dbConfig.ID)
				}

				cfg.Databases = append(cfg.Databases, dbConfig)
				added++
			}

			if added == 0 {
				fmt.Println("No databases added.")
				return nil
			}

			if err := config.SaveConfig(cfg, cfg.Paths.ConfigFile); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("Added %d database(s).\n", added)
			return nil
		},
	}
}
